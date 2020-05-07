package lib

import (
	"github.com/pkg/errors"
	"github.com/twmb/algoimpl/go/graph"
)

// DAG is responsible to gather all the components and their dependencies,
// and then returns a list of components sorted from no dependencies to
// the one with most dependencies. In that way we can prioritize creating the
// components and ensuring it is not going to fail because of lack of dependent object.

// Key for a component
type Key interface{}

// Value is the actual component object
type Value interface{}

// Graph is collection of vertices and edges between them.
// In dag, sort implement a topological sort for the graph.
type Graph interface {
	// AddVertex creates a new vertex in the graph with the specified key and stores
	// value provided in the vertex. It returns an error if the vertex specified by
	// the key already exists.
	AddVertex(Key, Value) error

	// RemoveVertex removes the vertex specified by the key. It returns an error
	// if the vertex corresponding to the key is not present in the graph.
	RemoveVertex(Key) error

	// AddDependencies creates a dependency between a given vertex and the provided
	// list of dependency vertices. This returns an error if either the vertex or the
	// dependency is not present in the graph or a make a node depends on itself.
	// Adding an dependency that creates a cycle in the graph is not allowed.
	AddDependencies(Key, ...Key) error

	// GetValue returns the value of the vertex specified by the key. It returns nil if the
	// vertex is not present in the graph.
	GetValue(Key) Value

	// SetValue sets the value of the vertex specified by the key. It returns an error if
	// the vertex is not present in the graph
	SetValue(Key, Value) error

	// Sort returns all the vertex entries in the dependency order. Vertices are ordered in
	// such a way that a vertex's dependencies will always preseed itself.
	Sort() []Vertex
}

// NewDAG creates a new DAG.
func NewDAG() Graph {
	return &dag{
		graph.New(graph.Directed),
		make(map[Key]graph.Node),
	}
}

// Vertex in the graph has the entry of <key, value> for a component
type Vertex struct {
	Key   Key
	Value Value
}

// dag is a Graph which has an internal graph.Graph inside itself handling
// nodes creation and making edges. It also has a map of all vertices which associates
// the key to a graph.Node that is holding their corresponding component.
type dag struct {
	graph    *graph.Graph
	vertices map[Key]graph.Node
}

func (dg *dag) AddVertex(key Key, val Value) error {
	if _, ok := dg.vertices[key]; ok {
		return errors.Errorf("key %s already exists", key)
	}
	node := dg.graph.MakeNode()
	*node.Value = &Vertex{key, val}
	dg.vertices[key] = node
	return nil
}

func (dg *dag) RemoveVertex(key Key) error {
	v, ok := dg.vertices[key]
	if !ok {
		return errors.Errorf("key %s does not exist", key)
	}
	dg.graph.RemoveNode(&v)
	delete(dg.vertices, key)
	return nil
}

func (dg *dag) AddDependencies(vertex Key, dependencies ...Key) error {
	for _, dep := range dependencies {
		srcObj, ok := dg.vertices[vertex]
		if !ok {
			return errors.Errorf("key %s does not exist", vertex)
		}

		if err := dg.addDep(srcObj, vertex, dep); err != nil {
			return err
		}
	}
	return nil
}

// adds a single dependency to the graph
func (dg *dag) addDep(srcObj graph.Node, node Key, dependency Key) error {
	dstObj, ok := dg.vertices[dependency]
	if !ok {
		return errors.Errorf("key %s does not exist", dependency)
	}

	if node == dependency {
		return errors.Errorf("edge to self is not allowed")
	}

	if err := dg.graph.MakeEdge(dstObj, srcObj); err != nil {
		return err
	}

	if !isAcyclic(dg) {
		dg.graph.RemoveEdge(dstObj, srcObj)
		return errors.Errorf("edge from %s to %s makes a cycle", node, dependency)
	}
	return nil

}

// Return the vertex by its key if exists else return nil.
func (dg *dag) GetValue(v Key) Value {
	if o, ok := dg.vertices[v]; ok {
		vertex := (*o.Value).(*Vertex)
		return vertex.Value
	}
	return nil
}

// Set the value of the vertex if exists else return error.
func (dg *dag) SetValue(v Key, val Value) error {
	if o, ok := dg.vertices[v]; ok {
		vertex := (*o.Value).(*Vertex)
		vertex.Value = val
		return nil
	}
	return errors.Errorf("key %s does not exist", v)
}

// A sorted traversal of this graph will guarantee the
// dependency order. This means A (node) depends on B (dependency) then
// the sorted traversal will always return B before A.
func (dg *dag) Sort() []Vertex {
	sorted := dg.graph.TopologicalSort()
	nodes := make([]Vertex, 0, len(sorted))
	for _, n := range sorted {
		vp := (*n.Value).(*Vertex)
		n := Vertex{vp.Key, vp.Value}
		nodes = append(nodes, n)
	}
	return nodes
}

// Determines whether a DAG is acyclic or not.
func isAcyclic(dg *dag) bool {
	connectedComponents := dg.graph.StronglyConnectedComponents()
	// If the arrays underlying each node has a size of one, it means that each
	// vertex in the dag is a connected component. There's not any connected component
	// with more than one vertex in it. Therefore there isn't any cycle in the DAG.
	for _, arr := range connectedComponents {
		if len(arr) > 1 {
			return false
		}
	}
	return true
}
