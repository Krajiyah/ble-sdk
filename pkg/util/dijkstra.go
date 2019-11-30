package util

// Copied from: https://rosettacode.org/wiki/Dijkstra%27s_algorithm#Go
import (
	"container/heap"
	"strings"
)

// A priorityQueue implements heap.Interface and holds Items.
type priorityQueue struct {
	items []vertex
	// value to index
	m map[vertex]int
	// value to priority
	pr map[vertex]int
}

func (pq *priorityQueue) Len() int           { return len(pq.items) }
func (pq *priorityQueue) Less(i, j int) bool { return pq.pr[pq.items[i]] < pq.pr[pq.items[j]] }
func (pq *priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.m[pq.items[i]] = i
	pq.m[pq.items[j]] = j
}
func (pq *priorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(vertex)
	pq.m[item] = n
	pq.items = append(pq.items, item)
}
func (pq *priorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	pq.m[item] = -1
	pq.items = old[0 : n-1]
	return item
}

// update modifies the priority of an item in the queue.
func (pq *priorityQueue) update(item vertex, priority int) {
	pq.pr[item] = priority
	heap.Fix(pq, pq.m[item])
}
func (pq *priorityQueue) addWithPriority(item vertex, priority int) {
	heap.Push(pq, item)
	pq.update(item, priority)
}

const (
	Infinity      = int(^uint(0) >> 1)
	Uninitialized = -1
)

func dijkstra(g graph, source vertex) (dist map[vertex]int, prev map[vertex]vertex) {
	dist = make(map[vertex]int)
	prev = make(map[vertex]vertex)
	sid := source
	dist[sid] = 0
	q := &priorityQueue{[]vertex{}, make(map[vertex]int), make(map[vertex]int)}
	for _, v := range g.Vertices() {
		if v != sid {
			dist[v] = Infinity
		}
		prev[v] = Uninitialized
		q.addWithPriority(v, dist[v])
	}
	for len(q.items) != 0 {
		u := heap.Pop(q).(vertex)
		for _, v := range g.Neighbors(u) {
			alt := dist[u] + g.Weight(u, v)
			if alt < dist[v] {
				dist[v] = alt
				prev[v] = u
				q.update(v, alt)
			}
		}
	}
	return dist, prev
}

// A graph is the interface implemented by graphs that
// this algorithm can run on.
type graph interface {
	Vertices() []vertex
	Neighbors(v vertex) []vertex
	Weight(u, v vertex) int
}

// Nonnegative integer ID of vertex
type vertex int

// sg is a graph of strings that satisfies the graph interface.
type sg struct {
	ids   map[string]vertex
	names map[vertex]string
	edges map[vertex]map[vertex]int
}

func newsg(ids map[string]vertex) sg {
	g := sg{ids: ids}
	g.names = make(map[vertex]string)
	for k, v := range ids {
		g.names[v] = k
	}
	g.edges = make(map[vertex]map[vertex]int)
	return g
}
func (g sg) edge(u, v string, w int) {
	if _, ok := g.edges[g.ids[u]]; !ok {
		g.edges[g.ids[u]] = make(map[vertex]int)
	}
	g.edges[g.ids[u]][g.ids[v]] = w
}
func (g sg) path(v vertex, prev map[vertex]vertex) []string {
	delim := "____"
	s := g.names[v]
	for prev[v] >= 0 {
		v = prev[v]
		s = g.names[v] + delim + s
	}
	return strings.Split(s, delim)
}
func (g sg) Vertices() (vs []vertex) {
	for _, v := range g.ids {
		vs = append(vs, v)
	}
	return vs
}
func (g sg) Neighbors(u vertex) (vs []vertex) {
	for v := range g.edges[u] {
		vs = append(vs, v)
	}
	return vs
}
func (g sg) Weight(u, v vertex) int { return g.edges[u][v] }

func ShortestPath(graph map[string]map[string]int, src, dst string) ([]string, error) {
	var counter vertex = 1
	verts := map[string]vertex{}
	for k, v := range graph {
		if _, ok := verts[k]; !ok {
			verts[k] = counter
			counter++
		}
		for key, _ := range v {
			if _, ok := verts[key]; !ok {
				verts[key] = counter
				counter++
			}
		}
	}
	g := newsg(verts)
	for k, v := range graph {
		for key, vv := range v {
			g.edge(k, key, vv)
		}
	}
	_, prev := dijkstra(g, g.ids[src])
	return g.path(g.ids[dst], prev), nil
}
