package etl

import "encoding/json"

// The property that says where an entity is.
//
// It is the edge of the administrative hierarchy: a city is located in
// a department, the department is located in a region, and the region
// is located in a country.
const PropertyLocatedIn = "P131"

// Hierarchy collects the administrative hierarchy of the dump.
//
// The hierarchy is built during the pass over the dump: every P131
// statement is recorded as an edge, and the edges are then walked from
// the target once the pass is over.
//
// It holds the edges and the identifiers of the candidates, and nothing
// else. The entities themselves stay in the items file: a run over the
// full dump writes hundreds of thousands of them, and holding them all
// in memory would cost more than the hierarchy does.
//
// The identifiers are held because they are what the walk works on. The
// walk is a breadth first search over the children of each entity, and
// it needs to know which entities are inside the target.
type Hierarchy struct {
	// children holds the entities located in each entity.
	//
	// It is keyed by the identifier of the parent, and the value is
	// the identifiers of the entities inside it. An entity can be
	// located in several places, and a place holds many entities, so
	// both sides are many to many.
	children map[string][]string
	// candidates holds the identifiers of the entities that were
	// written to the items file.
	//
	// An entity that was written is a candidate: the walk may reach
	// it, and it has to be kept if it does.
	candidates map[string]struct{}
}

// NewHierarchy creates an empty hierarchy.
func NewHierarchy() *Hierarchy {
	return &Hierarchy{
		children:   make(map[string][]string),
		candidates: make(map[string]struct{}),
	}
}

// Add records an item and the entities it is located in.
//
// The item is a candidate whatever the walk will decide about it: the
// walk happens after the pass, and the items it reaches are kept then.
// An item that is not reached is filtered out of the items file.
func (h *Hierarchy) Add(entity Entity) {
	h.candidates[entity.ID] = struct{}{}

	raw, ok := entity.Claims[PropertyLocatedIn]
	if !ok {
		return
	}

	for _, parent := range locatedIn(raw) {
		h.addEdge(entity.ID, parent)
	}
}

// addEdge records that child is located in parent.
func (h *Hierarchy) addEdge(child, parent string) {
	h.children[parent] = append(h.children[parent], child)
}

// locatedIn reads the entities a statement points at.
//
// A statement that cannot be read is not an edge: the dump is large
// and a damaged statement is rare, but it must not stop the pass.
func locatedIn(raw json.RawMessage) []string {
	var statements []rawStatement
	if err := json.Unmarshal(raw, &statements); err != nil {
		return nil
	}

	parents := make([]string, 0, len(statements))

	for _, statement := range statements {
		snak := statement.MainSnak
		if snak == nil || snak.SnakeType != SnakeTypeValue {
			continue
		}
		if snak.Datatype != DatatypeItem {
			continue
		}
		if snak.DataValue == nil {
			continue
		}

		var value rawEntityValue
		if err := json.Unmarshal(snak.DataValue.Value, &value); err != nil {
			continue
		}
		if value.ID == "" {
			continue
		}

		parents = append(parents, value.ID)
	}

	return parents
}

// Reachable returns the identifiers of the entities that are inside the
// target.
//
// The walk starts at the target and goes down the administrative
// hierarchy: every entity that is located in the target, and every
// entity that is located in one of those, and so on.
//
// The target is in the result: it is inside itself.
//
// The visited set keeps the walk from looping: a hierarchy can hold a
// cycle, and the walk must not follow it forever.
func (h *Hierarchy) Reachable(target string) map[string]struct{} {
	visited := make(map[string]struct{})
	visited[target] = struct{}{}

	queue := []string{target}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, child := range h.children[current] {
			if _, seen := visited[child]; seen {
				continue
			}
			visited[child] = struct{}{}
			queue = append(queue, child)
		}
	}

	return visited
}

// Candidate tells whether an identifier was written to the items file.
//
// Only a candidate can be kept: an entity that was never written
// cannot be put back into the file.
func (h *Hierarchy) Candidate(id string) bool {
	_, ok := h.candidates[id]
	return ok
}

// Len returns the number of candidates the hierarchy holds.
func (h *Hierarchy) Len() int {
	return len(h.candidates)
}

// Edges returns the number of edges the hierarchy holds.
//
// It is the number of pairs of entities that are linked by a P131
// statement.
func (h *Hierarchy) Edges() int {
	count := 0
	for _, children := range h.children {
		count += len(children)
	}
	return count
}
