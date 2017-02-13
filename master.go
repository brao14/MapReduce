package mapreduce

import (
	"fmt"
	"net"
	"sync"
)

// Master holds all the states that the master needs to keep track off
type Master struct {
	sync.Mutex

	address string
	registerChannel chan string
	doneChannel chan bool
	workers []string //protected by mutex

	// Per-task information
	jobName string //name of the currently executing job
	files []string //input files
	nReduce int //number of reduce partitions

	shutdown chan struct{}
	l net.Listener
	stats []int
}

// newMaster initialize a new Map/Reduce Master
func newMaster(master string) (mr *Master) {
	mr = new(Master)
	mr.address = master
	mr.shutdown = make(chan struct{})
	mr.registerChannel = make(chan string)
	mr.doneChannel = make(chan bool)
	return
}
// Sequential runs map and reduce tasks sequentially, waiting for 
// each task to finish before scheduling the next
func Sequential(jobName string, files []string, nreduce int,
	mapF func(string, string) []KeyValue,
	reduceF func(string, []string) string,
) (mr *Master) {
	mr = newMaster("master")
	go mr.run(jobName, files, nreduce, func(phase jobPhase) {
		switch phase {
		case mapPhase:
			for i, f := range files {
				doMap(mr.jobName, i, f, mr.nReduce, mapF)
			}
		case reducePhase:
			for i := 0; i < mr.nReduce; i++ {
				doReduce(mr.jobName, i, len(mr.files), reduceF)
			}
		}
		}, func() {
			mr.stats = []int{len(files) + nreduce}
			})
	return
}

// run executes a mapreduce job on the given number of mappers and reducers
//
func (mr *Master) run(jobName string, files []string, nreduce int,
	schedule func(phase jobPhase),
	finish func(),
) {
	mr.jobName = jobName
	mr.files = files
	mr.nReduce = nreduce

	fmt.Printf("%s: starting Map/Reduce task %s\n", mr.address, mr.jobName)

	schedule(mapPhase)
	schedule(reducePhase)
	finish()
	mr.merge()

	fmt.Printf("%s: Map/Reduce task completed\n", mr.address)

	mr.doneChannel <- true
}

// Wait blocks until the currently scheduled work has completed.
// This happens when all tasks have scheduled and completed, the final output
// have been computed, and all workers have been shut down.
func (mr *Master) Wait() {
	<-mr.doneChannel
}