package compoapp

import (
	"fmt"
	"reflect"
	"sync"
)

// Container holds and manages dependencies
type Container struct {
	// Registry of constructors
	constructors map[string]*constructorInfo
	// Resolved instances
	instances map[string]any
	// Lock for thread safety
	mu sync.RWMutex
	// Graph for dependency resolution
	graph *dependencyGraph
}

// constructorInfo holds constructor function and metadata
type constructorInfo struct {
	fn          any
	name        string
	dependsOn   []string
	returnTypes []reflect.Type
}

// dependencyGraph represents the dependency relationships
type dependencyGraph struct {
	dependencies map[string][]string // component -> its dependencies
	dependents   map[string][]string // component -> components that depend on it
}

// NewContainer creates a new DI container
func NewContainer() *Container {
	return &Container{
		constructors: make(map[string]*constructorInfo),
		instances:    make(map[string]any),
		graph: &dependencyGraph{
			dependencies: make(map[string][]string),
			dependents:   make(map[string][]string),
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

	// Analyze function signature
	dependsOn, returnTypes, err := c.analyzeFunction(constructorType)
	if err != nil {
		return fmt.Errorf("failed to analyze constructor: %w", err)
	}

	// Use first return type as name (you might want a better naming strategy)
	if len(returnTypes) == 0 {
		return fmt.Errorf("constructor must return at least one value")
	}

	// Generate name from return type
	name := c.generateName(returnTypes[0])

	// Store constructor info
	c.constructors[name] = &constructorInfo{
		fn:          constructor,
		name:        name,
		dependsOn:   dependsOn,
		returnTypes: returnTypes,
	}

	// Update dependency graph
	c.graph.dependencies[name] = dependsOn
	for _, dep := range dependsOn {
		c.graph.dependents[dep] = append(c.graph.dependents[dep], name)
	}

	return nil
}

// ProvideNamed registers a constructor with explicit name but panic on error
func (c *Container) MustProvideNamed(name string, constructor any) {
	if err := c.ProvideNamed(name, constructor); err != nil {
		panic(err)
	}
}

// ProvideNamed registers a constructor with explicit name
func (c *Container) ProvideNamed(name string, constructor any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	constructorValue := reflect.ValueOf(constructor)
	if constructorValue.Kind() != reflect.Func {
		return fmt.Errorf("constructor must be a function")
	}

	constructorType := constructorValue.Type()

	// Analyze function signature
	dependsOn, returnTypes, err := c.analyzeFunction(constructorType)
	if err != nil {
		return fmt.Errorf("failed to analyze constructor: %w", err)
	}

	// Store constructor info
	c.constructors[name] = &constructorInfo{
		fn:          constructor,
		name:        name,
		dependsOn:   dependsOn,
		returnTypes: returnTypes,
	}

	// Update dependency graph
	c.graph.dependencies[name] = dependsOn
	for _, dep := range dependsOn {
		c.graph.dependents[dep] = append(c.graph.dependents[dep], name)
	}

	return nil
}

// analyzeFunction extracts dependencies and return types from function signature
func (c *Container) analyzeFunction(fnType reflect.Type) ([]string, []reflect.Type, error) {
	var dependsOn []string
	var returnTypes []reflect.Type

	// Analyze parameters (dependencies)
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		// Generate dependency name from parameter type
		depName := c.generateName(paramType)
		dependsOn = append(dependsOn, depName)
	}

	// Analyze return values
	for i := 0; i < fnType.NumOut(); i++ {
		returnTypes = append(returnTypes, fnType.Out(i))
	}

	// Check if last return value is error
	if len(returnTypes) > 0 && returnTypes[len(returnTypes)-1].String() == "error" {
		// Remove error from return types for naming purposes
		returnTypes = returnTypes[:len(returnTypes)-1]
	}

	return dependsOn, returnTypes, nil
}

// generateName creates a name from type
func (c *Container) generateName(t reflect.Type) string {
	// Handle pointers
	if t.Kind() == reflect.Pointer {
		return "*" + t.Elem().String()
	}
	return t.String()
}

// Resolve resolves and returns an instance of the requested type
func (c *Container) Resolve(target any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	targetType := targetValue.Type().Elem()
	name := c.generateName(targetType)

	// Build dependency graph and sort
	sortedNames, err := c.topologicalSort()
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Resolve all dependencies in order
	for _, name := range sortedNames {
		if _, exists := c.instances[name]; !exists {
			if err := c.resolveInstance(name); err != nil {
				return fmt.Errorf("failed to resolve %s: %w", name, err)
			}
		}
	}

	// Set the target value
	if instance, exists := c.instances[name]; exists {
		instanceValue := reflect.ValueOf(instance)
		if instanceValue.Type().AssignableTo(targetType) {
			targetValue.Elem().Set(instanceValue)
			return nil
		}
		return fmt.Errorf("resolved instance type %s is not assignable to target type %s",
			instanceValue.Type(), targetType)
	}

	return fmt.Errorf("no instance found for type %s", name)
}

// topologicalSort performs topological sort on dependency graph
func (c *Container) topologicalSort() ([]string, error) {
	// Kahn's algorithm for topological sorting
	inDegree := make(map[string]int)

	// Initialize in-degrees
	for name := range c.constructors {
		inDegree[name] = 0
	}

	// Calculate in-degrees
	for name, deps := range c.graph.dependencies {
		for _, dep := range deps {
			if _, exists := c.constructors[dep]; !exists {
				return nil, fmt.Errorf("missing dependency: %s", dep)
			}
			inDegree[name]++
		}
	}

	// Find nodes with zero in-degree
	queue := []string{}
	result := []string{}

	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Process nodes
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for dependents
		for _, dependent := range c.graph.dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(c.constructors) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// resolveInstance creates an instance for a given name
func (c *Container) resolveInstance(name string) error {
	constructorInfo, exists := c.constructors[name]
	if !exists {
		return fmt.Errorf("no constructor registered for %s", name)
	}

	constructorValue := reflect.ValueOf(constructorInfo.fn)
	constructorType := constructorValue.Type()

	// Prepare arguments
	args := make([]reflect.Value, constructorType.NumIn())

	for i := 0; i < constructorType.NumIn(); i++ {
		paramType := constructorType.In(i)
		depName := c.generateName(paramType)

		// Get dependency instance
		depInstance, depExists := c.instances[depName]
		if !depExists {
			return fmt.Errorf("dependency %s not resolved for %s", depName, name)
		}

		args[i] = reflect.ValueOf(depInstance)
	}

	// Call constructor
	results := constructorValue.Call(args)

	// Handle error return
	if len(results) > 0 {
		lastResult := results[len(results)-1]
		if lastResult.Type().String() == "error" && !lastResult.IsNil() {
			return lastResult.Interface().(error)
		}
	}

	// Store first return value as instance
	if len(results) > 0 {
		c.instances[name] = results[0].Interface()
	}

	return nil
}

// MustResolve is like Resolve but panics on error
func (c *Container) MustResolve(target any) {
	if err := c.Resolve(target); err != nil {
		panic(err)
	}
}
