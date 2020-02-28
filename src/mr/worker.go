package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)
import "log"
import "net/rpc"
import "hash/fnv"


//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }


//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	//fmt.Println("Entering map...")
	for CallExample() == false {
		time.Sleep(time.Second)
	}
	for {
		if ret := checkMapFinish(); ret {
			break
		}
		mapTaskIndex, mapTaskFilename := getMapTask()
		//fmt.Println(mapTaskIndex, mapTaskFilename)

		file, err := os.Open(mapTaskFilename)
		if err != nil {
			log.Fatalf("cannot open %v", mapTaskFilename)
		}
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", mapTaskFilename)
		}
		file.Close()
		kva := mapf(mapTaskFilename, string(content))

		nReduce := getnReduce()
		intermediateHandler := make([]*json.Encoder, nReduce)
		for i := 0; i < nReduce; i++ {
			fileHandler, _ := os.Create(
				fmt.Sprintf("mrtest-%d-%d.txt", mapTaskIndex, i))
			intermediateHandler[i] = json.NewEncoder(fileHandler)
		}
		for _, kv := range kva {
			n := ihash(kv.Key) % nReduce
			_ = intermediateHandler[n].Encode(&kv)
		}
		intermediateHandler = nil
		kva = nil

		callMapFinished(mapTaskIndex)
		time.Sleep(time.Second)
	}

	//fmt.Println("Entering reduce...")
	for {
		if ret := checkReduceFinish(); ret {
			break
		}
		reduceTaskIndex := getReduceTask()
		//fmt.Println(reduceTaskIndex)
		var intermediate []KeyValue

		var reduceFiles []string
		f, _ := os.Open(".")
		files, _ := f.Readdir(-1)
		f.Close()
		for _, file := range files {
			if strings.HasSuffix(file.Name(), strconv.Itoa(reduceTaskIndex) + ".txt") {
				reduceFiles = append(reduceFiles, file.Name())
			}
		}
		//fmt.Println(reduceFiles)

		for _, v := range reduceFiles {
			tFile, _ := os.Open(v)
			dec := json.NewDecoder(tFile)
			for {
				var kv KeyValue
				if err := dec.Decode(&kv); err != nil {
					tFile.Close()
					break
				}
				intermediate = append(intermediate, kv)
			}
		}
		//fmt.Println("len of intermediate: ", len(intermediate))

		sort.Sort(ByKey(intermediate))

		otfile, _ := ioutil.TempFile("", "reduce" + strconv.Itoa(reduceTaskIndex))
		i := 0
		for i < len(intermediate) {
			j := i + 1
			for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
				j++
			}
			values := []string{}
			for k := i; k < j; k++ {
				values = append(values, intermediate[k].Value)
			}
			output := reducef(intermediate[i].Key, values)

			// this is the correct format for each line of Reduce output.
			fmt.Fprintf(otfile, "%v %v\n", intermediate[i].Key, output)

			i = j
		}
		_ = os.Rename(otfile.Name(), "./mr-out-" + strconv.Itoa(reduceTaskIndex))
		otfile.Close()
		callReduceFinished(reduceTaskIndex)
		time.Sleep(time.Second)
	}


	// uncomment to send the Example RPC to the master.
	// CallExample()

}

func getMapTask() (int, string){
	args := GetMapTaskArgs{}
	reply := GetMapTaskReply{}
	call("Master.GetMapTask", &args, &reply)
	return reply.Index, reply.Filename
}

func getnReduce() int {
	args := GetnReduceArgs{}
	reply := GetnReduceReply{}
	call("Master.GetnReduce", &args, &reply)
	return reply.NReduce
}

func callMapFinished(index int) {
	args := UpdateStatusArgs{Index: index}
	reply := UpdateStatusReply{}
	call("Master.UpdateMapTaskStatus", &args, &reply)
}

func getReduceTask() int {
	args := GetReduceTaskArgs{}
	reply := GetReduceTaskReply{}
	call("Master.GetReduceTask", &args, &reply)
	return reply.Index
}

func callReduceFinished(index int) {
	args := UpdateStatusArgs{Index: index}
	reply := UpdateStatusReply{}
	call("Master.UpdateReduceTaskStatus", &args, &reply)
}

func checkMapFinish() bool {
	args := CheckJobStatusArgs{}
	reply := CheckJobStatusReply{}
	call("Master.CheckMapJobStatus", &args, &reply)
	return reply.IfFinished
}

func checkReduceFinish() bool {
	args := CheckJobStatusArgs{}
	reply := CheckJobStatusReply{}
	ret := call("Master.CheckReduceJobStatus", &args, &reply)
	return reply.IfFinished || !ret
}

//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() bool {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	return call("Master.Example", &args, &reply)
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
