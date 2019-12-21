package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
	"container/heap"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"strings"
	"strconv"
)

// An Item is something we manage in a priority queue.
type Item struct {
	value string // The value of the item; arbitrary.
	priority time.Time // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

func (item Item) Value() string { return item.value; }
func (item Item) Priority() time.Time { return item.priority; }

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority.UnixNano() < pq[j].priority.UnixNano()
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, value string, priority time.Time) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}

func main() {
	// https://stackoverflow.com/questions/31706893/golang-microseconds-in-log-package
	// log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	nodes := 1

	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	logs := make(map[string]*PriorityQueue)

	for i := 1; i <= nodes; i++ {
		filename := fmt.Sprintf("node%03d.log", i)
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := scanner.Text()
			broken := strings.SplitN(text, " ", 3) // 6
			timestamp := broken[0] + " " + broken[1]
			payload := broken[2]
			pieces := strings.SplitN(payload, " ", 8)
			uid := pieces[3]
			if strings.Contains(pieces[4], ".") {

				pq := make(PriorityQueue, 0)
				queue := &pq

				val, ok := logs[uid]
				if ok {
					queue = val
				} else {
					logs[uid] = queue
				}

				t, _ := time.Parse("2006/01/02 15:04:05.999999", timestamp)
				item := &Item{
					value: payload,
					priority: t,
				}
				heap.Push(queue, item)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

// 	// Take the items out; they arrive in decreasing priority order.
// 	for pq.Len() > 0 {
// 		item := heap.Pop(&pq).(*Item)
// 		fmt.Printf("%.2d:%s ", item.priority, item.value)
// 	}
// }


	min := make(plotter.XYs, 0)
	median := make(plotter.XYs, 0)
	max := make(plotter.XYs, 0)
	nums := make(plotter.XYs, 0)

	for k, v := range logs {
		knum, _ := strconv.ParseInt(k[0:16], 16, 64)
		kfloat := float64(knum)
		n := v.Len()
		nums = append(nums, plotter.XY{kfloat,float64(n)})
		for i := 0; i < n; i++ {
			item := *v.Pop().(*Item)
			if i == 0 || i == n/2 || i == n - 1 {
				pieces := strings.SplitN(item.Value(), " ", 8)
				timestrs := strings.SplitN(pieces[4], ".", 3)
				sec, _ := strconv.ParseInt(timestrs[0], 10, 64)
				nsec, _ := strconv.ParseInt(timestrs[1], 10, 64)
				starttime := time.Unix(sec, nsec * 1000)
				elapsed := item.Priority().Sub(starttime).Nanoseconds()
				// direction := broken[2]
				// remote := broken[3]
				// category := broken[4]
				// message = broken[5]
				if i == 0 {
					min = append(min, plotter.XY{kfloat,float64(elapsed)/1000000000})
				} else if i == n / 2 {
					median = append(median, plotter.XY{kfloat,float64(elapsed)/1000000000})
				} else {
					max = append(max, plotter.XY{kfloat,float64(elapsed)/1000000000})
				}
			}
		}
	}

	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Spread of transaction"
	p.X.Label.Text = "Time"
	p.Y.Label.Text = "Percentage of nodes spread to"

	err = plotutil.AddScatters(p,
		"Min", min,
		"Median", median,
		"Max", max)
	if err != nil {
		panic(err)
	}

	// Save the plot to a PNG file.
	if err := p.Save(4*vg.Inch, 4*vg.Inch, "propdelay.png"); err != nil {
		panic(err)
	}
}
// 	runtime.Goexit()
// 	fmt.Println("Exit")
// }
