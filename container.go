// container.go
package compoapp

import (
	"fmt"
	"reflect"
	"sync"
)

// Container holds and manages dependencies
type Container struct {
	// list of constructors
	constructors []*constructorInfo
	// Resolved instances
	instances map[reflect.Type]any
	// Registry of types. All types which returned from ctors
	typeRegistry []reflect.Type
	// Lock for thread safety
	mu sync.RWMutex
	// Graph for dependency resolution
	graph *dependencyGraph
	// ctor for specific
	typesCtors map[reflect.Type]*constructorInfo

	debug bool
}

// Debug enables debug mode
func (c *Container) Debug() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.debug = true
}

func (c *Container) debugf(format string, args ...any) {
	if c.debug {
		fmtStr := "[CONTAINER] " + format + "\n"
		fmt.Printf(fmtStr, args...)
	}
}

// fnSignature - describes function args and return values
// todo: for now we only support one return value
type fnSignature struct {
	args       []reflect.Type
	returnType reflect.Type
}

// constructorInfo holds constructor function and metadata
type constructorInfo struct {
	fn        any
	name      string
	signature fnSignature
	// New fields for interface resolution
	dependNeedsResolution []bool // marks which dependencies need interface resolution
}

// dependencyGraph represents the dependency relationships
type dependencyGraph struct {
	dependencies map[reflect.Type][]reflect.Type // component -> its dependencies
	dependents   map[reflect.Type][]reflect.Type // component -> components that depend on it
}

// NewContainer creates a new DI container
func NewContainer() *Container {
	return &Container{
		constructors: []*constructorInfo{},
		instances:    make(map[reflect.Type]any),
		typeRegistry: []reflect.Type{},
		typesCtors:   make(map[reflect.Type]*constructorInfo),
		graph: &dependencyGraph{
			dependencies: make(map[reflect.Type][]reflect.Type),
			dependents:   make(map[reflect.Type][]reflect.Type),
		},
	}
}

// MustProvide registers a constructor function and panic on error
func (c *Container) MustProvide(constructor any) {
	if err := c.Provide(constructor); err != nil {
		panic(err)
	}
}

// Provide registers a constructor function
func (c *Container) Provide(constructor any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	constructorValue := reflect.ValueOf(constructor)
	if constructorValue.Kind() != reflect.Func {
		return fmt.Errorf("constructor must be a function")
	}

	constructorType := constructorValue.Type()
	c.debugf("provided constructor %s", constructorType.String())
	// Analyze function signature
	signature, err := c.analyzeFunction(constructorType)
	if err != nil {
		return fmt.Errorf("failed to analyze constructor: %w", err)
	}

	// Initialize dependency resolution tracking
	dependNeedsResolution := make([]bool, len(signature.args))
	for i, arg := range signature.args {
		// Mark interfaces for resolution
		if arg.Kind() == reflect.Interface {
			dependNeedsResolution[i] = true
		}
	}

	// Store constructor info
	cinfo := &constructorInfo{
		fn:                    constructor,
		name:                  constructorType.String(),
		signature:             signature,
		dependNeedsResolution: dependNeedsResolution,
	}
	c.constructors = append(c.constructors, cinfo)
	// todo: only one return value available right now
	c.typesCtors[signature.returnType] = cinfo

	// Register return types in type registry for interface resolution
	returnType := signature.returnType
	c.typeRegistry = append(c.typeRegistry, returnType)
	// todo: somehow we should find out that we have pointer, reference and values

	return nil
}

// analyzeFunction extracts dependencies and return types from function signature
func (c *Container) analyzeFunction(fnType reflect.Type) (fnSignature, error) {
	c.debugf("analyzing constructor %s signature", fnType.String())

	args := make([]reflect.Type, 0, fnType.NumIn())

	// Analyze args (dependencies)
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		// Generate dependency name from parameter type
		c.debugf("arg: %d, type: %s", i, paramType.String())

		args = append(args, paramType)
	}

	// Analyze return values
	// Support either: (*T) or (*T, error)
	if fnType.NumOut() == 0 || fnType.NumOut() > 2 {
		return fnSignature{}, fmt.Errorf("constructor must return (*T) or (*T, error)")
	}

	firstOut := fnType.Out(0)
	if firstOut.Kind() != reflect.Pointer {
		return fnSignature{}, fmt.Errorf("constructor must return pointer value as first result")
	}

	if fnType.NumOut() == 2 {
		secondOut := fnType.Out(1)
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		if !secondOut.Implements(errorType) {
			return fnSignature{}, fmt.Errorf("second return value must be error")
		}
	}

	return fnSignature{args, firstOut}, nil
}

// Resolve resolves and returns an instance of the requested type.
// Target must be a pointer to a pointer.
func (c *Container) Resolve(target any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	targetType := targetValue.Type().Elem()

	// Step 1: Resolve interfaces to implementations
	if err := c.resolveInterfaces(); err != nil {
		return fmt.Errorf("interface resolution failed: %w", err)
	}

	c.rebuildGraph()

	if err := c.validateDependencies(); err != nil {
		return err
	}

	// Step 2: Build dependency graph and sort
	sortedTypes, err := c.topologicalSort()
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Step 3: Resolve all dependencies in order
	for _, name := range sortedTypes {
		// todo: here might be tagged instances too
		if err := c.resolveInstance(name); err != nil {
			return fmt.Errorf("failed to resolve %s: %w", name, err)
		}
	}

	// Step 4: Set the target value
	if instance, exists := c.instances[targetType]; exists {
		instanceValue := reflect.ValueOf(instance)
		if instanceValue.Type().AssignableTo(targetType) {
			targetValue.Elem().Set(instanceValue)
			return nil
		}
		return fmt.Errorf("resolved instance type %s is not assignable to target type %s",
			instanceValue.Type(), targetType)
	}

	return fmt.Errorf("no instance found for type %s", targetType)
}

// resolveInterfaces resolves interface dependencies to concrete implementations
func (c *Container) resolveInterfaces() error {
	c.debugf("resolving interfaces")
	// For each constructor, check if it has interface dependencies that need resolution
	for _, ctorInfo := range c.constructors {
		for i, needsResolution := range ctorInfo.dependNeedsResolution {
			if !needsResolution {
				continue
			}

			signature := &ctorInfo.signature

			interfaceType := signature.args[i]

			// Find implementation
			implementations := c.findImplementations(interfaceType)
			if len(implementations) == 0 {
				return fmt.Errorf("no implementation found for interface %s", interfaceType.String())
			}
			if len(implementations) > 1 {
				return fmt.Errorf("multiple implementations found for interface %s: %v",
					interfaceType.String(), implementations)
			}

			// Replace interface dependency with concrete implementation
			c.debugf("%s replaced with implementation %s", signature.args[i], implementations[0])
			signature.args[i] = implementations[0]
		}
	}
	return nil
}

// findImplementations finds concrete implementations for an interface type
func (c *Container) findImplementations(interfaceType reflect.Type) []reflect.Type {
	c.debugf("searching implementation for %s", interfaceType)
	var implementations []reflect.Type

	// For interface types, look for concrete implementations
	for _, typ := range c.typeRegistry {
		// todo: may be just return error at the Provide stage?
		if typ.Kind() == reflect.Interface {
			continue
		}
		c.debugf("checking %s", typ)
		// Check direct implementation
		if typ.Implements(interfaceType) {
			implementations = append(implementations, typ)
			c.debugf("%s implements %s", typ, interfaceType)
			continue
		}
		// Check pointer implementation
		if reflect.PointerTo(typ).Implements(interfaceType) {
			implementations = append(implementations, typ)
			c.debugf("%s implements %s", typ, interfaceType)
		}
	}
	c.debugf("found %d implementations", len(implementations))

	return implementations
}

// topologicalSort performs topological sort on dependency graph
func (c *Container) topologicalSort() ([]reflect.Type, error) {
	// Kahn's algorithm for topological sorting
	c.debugf("started topological sort")
	inDegree := make(map[reflect.Type]int)

	// Initialize in-degrees
	for _, typ := range c.typeRegistry {
		inDegree[typ] = 0
	}
	c.debugf("initialized in-degrees: %v", inDegree)

	// Calculate in-degrees
	for typ := range c.graph.dependencies {
		deps := c.graph.dependencies[typ]
		inDegree[typ] = len(deps) // Set the actual number of dependencies
		c.debugf("type %s has %d dependencies: %v", typ, len(deps), deps)
	}
	c.debugf("calculated in-degrees: %v", inDegree)

	// Find nodes with zero in-degree
	queue := []reflect.Type{}
	result := []reflect.Type{}

	for typ, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, typ)
			c.debugf("added to queue (zero in-degree): %s", typ)
		}
	}
	c.debugf("initial queue: %v", queue)

	// Process nodes
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		c.debugf("processing: %s", current)

		// Reduce in-degree for dependents
		for _, dependent := range c.graph.dependents[current] {
			inDegree[dependent]--
			c.debugf("reduced in-degree for %s: %d", dependent, inDegree[dependent])
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				c.debugf("added to queue: %s", dependent)
			}
		}
	}
	c.debugf("final result: %v", result)

	// Check for circular dependencies
	// Note: This should be equal to the number of types that have constructors
	typesWithConstructors := 0
	for range c.typesCtors {
		typesWithConstructors++
	}

	if len(result) != typesWithConstructors {
		return nil, fmt.Errorf("circular dependency detected: processed %d out of %d types", len(result), typesWithConstructors)
	}

	return result, nil
}

// resolveInstance creates an instance for a given type
func (c *Container) resolveInstance(typ reflect.Type) error {
	// check if we even have such type returned from ctors
	if _, exists := c.typesCtors[typ]; !exists {
		return fmt.Errorf("no constructor registered for %s", typ)
	}

	// find specific ctor which returns desired type
	// above we check that we have registered constructor, so no worries
	ctor := c.typesCtors[typ]

	constructorValue := reflect.ValueOf(ctor.fn)
	constructorType := constructorValue.Type()

	// Prepare arguments
	args := make([]reflect.Value, constructorType.NumIn())

	for i := 0; i < constructorType.NumIn(); i++ {
		depType := ctor.signature.args[i] // Use resolved dependency name

		// Get dependency instance
		depInstance, depExists := c.instances[depType]
		if !depExists {
			return fmt.Errorf("dependency %s not resolved for %s", depType, typ)
		}

		args[i] = reflect.ValueOf(depInstance)
	}

	// Call constructor
	results := constructorValue.Call(args)

	// Handle optional error return (when present and non-nil)
	if len(results) > 1 {
		lastResult := results[len(results)-1]
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		if lastResult.Type().Implements(errorType) && !lastResult.IsNil() {
			return lastResult.Interface().(error)
		}
	}

	// Store first return value as instance
	if len(results) > 0 {
		c.instances[typ] = results[0].Interface()
	}

	return nil
}

// MustResolve is like Resolve but panics on error.
func (c *Container) MustResolve(target any) {
	if err := c.Resolve(target); err != nil {
		panic(err)
	}
}

// rebuild the graph
func (c *Container) rebuildGraph() {
	c.debugf("rebuilding dependency graph after interface resolution")

	// Clear existing graph
	c.graph.dependencies = make(map[reflect.Type][]reflect.Type)
	c.graph.dependents = make(map[reflect.Type][]reflect.Type)

	// Rebuild based on resolved signatures
	for typ, ctor := range c.typesCtors {
		c.graph.dependencies[typ] = ctor.signature.args
		for _, dep := range ctor.signature.args {
			c.graph.dependents[dep] = append(c.graph.dependents[dep], typ)
		}
	}

	c.debugf("rebuilt dependencies: %v", c.graph.dependencies)
	c.debugf("rebuilt dependents: %v", c.graph.dependents)
}

// validateDependencies checks if all dependencies have corresponding constructors
func (c *Container) validateDependencies() error {
	c.debugf("validating dependencies")

	requiredTypes := make(map[reflect.Type]bool)

	// Get all dependencies from constructor signatures
	for _, ctor := range c.constructors {
		for _, depType := range ctor.signature.args {
			requiredTypes[depType] = true
		}
	}

	c.debugf("required dependency types: %v", requiredTypes)

	// Check if we have constructors for all required types
	for depType := range requiredTypes {
		if _, exists := c.typesCtors[depType]; !exists {
			return fmt.Errorf("missing constructor for dependency type: %s", depType.String())
		}
	}
	return nil
}
