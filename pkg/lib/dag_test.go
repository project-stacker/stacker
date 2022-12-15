package lib

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDag(t *testing.T) {
	Convey("Create a dag", t, func() {
		dag := NewDAG()
		So(dag, ShouldNotBeNil)

		// Assert that only unique vertices can be created
		So(dag.AddVertex("shirt", 1), ShouldBeNil)
		So(dag.AddVertex("tie", 2), ShouldBeNil)
		So(dag.AddVertex("belt", 3), ShouldBeNil)
		So(dag.AddVertex("pants", 4), ShouldBeNil)
		So(dag.AddVertex("jacket", 5), ShouldBeNil)
		So(dag.AddVertex("shirt", 6), ShouldBeError)

		// Assert that edges can be created to only existing vertices
		So(dag.AddDependencies("tie", "shirt"), ShouldBeNil)
		So(dag.AddDependencies("jacket", "tie", "belt"), ShouldBeNil)
		So(dag.AddDependencies("belt", "pants"), ShouldBeNil)
		So(dag.AddDependencies("shirt", "does_not_exist"), ShouldBeError)
		So(dag.AddDependencies("does_not_exist", "shirt"), ShouldBeError)

		// Assert that cycles cannot happen
		So(dag.AddDependencies("shirt", "shirt"), ShouldBeError)
		So(dag.AddDependencies("shirt", "jacket"), ShouldBeError)

		// Check if the vertex can be retrieved
		So(dag.GetValue("shirt"), ShouldEqual, 1)
		So(dag.GetValue("unknown_key"), ShouldBeNil)

		// Check if the vertex can be set
		So(dag.SetValue("shirt", 9), ShouldBeNil)
		So(dag.GetValue("shirt"), ShouldEqual, 9)
		So(dag.SetValue("unknown_key", 99), ShouldNotBeNil)

		expectedSortedNodes := []Vertex{
			{"pants", 4},
			{"belt", 3},
			{"shirt", 9},
			{"tie", 2},
			{"jacket", 5},
		}
		sortedNodes := dag.Sort()
		So(sortedNodes, ShouldResemble, expectedSortedNodes)

		// Add and remove a vertex
		So(dag.AddVertex("hat", 100), ShouldBeNil)
		So(dag.RemoveVertex("hat"), ShouldBeNil)
		So(dag.GetValue("hat"), ShouldBeNil)
		So(dag.RemoveVertex("unknown_thing"), ShouldBeError)
	})
}
