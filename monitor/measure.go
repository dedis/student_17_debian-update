package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"io/ioutil"
	"strconv"
	"strings"

	"github.com/dedis/cothority/log"
)

// Sink is the server address where all measures are transmitted to for
// further analysis.
var sink string

// Structs are encoded through a json encoder.
var encoder *json.Encoder
var connection net.Conn

// Keeps track if a measure is enabled (true) or not (false). If disabled,
// measures are not sent to the monitor. Use EnableMeasure(bool) to toggle
// this variable.
var enabled = true

// Measure is an interface for measurements
// Usage:
// 		measure := monitor.SingleMeasure("bandwidth")
// or
//		 measure := monitor.NewTimeMeasure("round")
// 		 measure.Record()
type Measure interface {
	// Record must be called when you want to send the value
	// over the monitor listening.
	// Implementation of this interface must RESET the value to `0` at the end
	// of Record(). `0` means the initial value / meaning this measure had when
	// created.
	// Example: TimeMeasure.Record() will reset the time to `time.Now()`
	//          CounterIOMeasure.Record() will  reset the counter of the bytes
	//          read / written to 0.
	//          etc
	Record()
}

// SingleMeasure is a pair name - value we want to send
type SingleMeasure struct {
	Name  string
	Value float64
}

// TimeMeasure represents a measure regarding time: It includes the wallclock
// time, the cpu time + the user time.
type TimeMeasure struct {
	Wall *SingleMeasure
	CPU  *SingleMeasure
	User *SingleMeasure
	// non exported fields
	// name of the time measure (basename)
	name string
	// last time
	lastWallTime time.Time
}

// ConnectSink connects to the given endpoint and initialises a json
// encoder. It can be the address of a proxy or a monitoring process.
// Returns an error if it could not connect to the endpoint.
func ConnectSink(addr string) error {
	if encoder != nil {
		return nil
	}
	log.Lvl3("Connecting to:", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	log.Lvl3("Connected to sink:", addr)
	sink = addr
	connection = conn
	encoder = json.NewEncoder(conn)
	return nil
}

// NewSingleMeasure returns a new measure freshly generated
func NewSingleMeasure(name string, value float64) *SingleMeasure {
	return &SingleMeasure{
		Name:  name,
		Value: value,
	}
}

// Record sends the value to the monitor. Reset the value to 0.
func (sm *SingleMeasure) Record() {
	if err := send(sm); err != nil {
		log.Error("Error sending SingleMeasure", sm.Name, " to monitor:", err)
	}
	sm.Value = 0
}

// NewTimeMeasure return *TimeMeasure
func NewTimeMeasure(name string) *TimeMeasure {
	tm := &TimeMeasure{name: name}
	tm.reset()
	return tm
}

// Record sends the measurements to the monitor:
//
// - wall time: *name*_wall
//
// - system time: *name*_system
//
// - user time: *name*_user
func (tm *TimeMeasure) Record() {
	// Wall time measurement
	tm.Wall = NewSingleMeasure(tm.name+"_wall", float64(time.Since(tm.lastWallTime))/1.0e9)
	// CPU time measurement
	tm.CPU.Value, tm.User.Value = getDiffRTime(tm.CPU.Value, tm.User.Value)
	// send data
	tm.Wall.Record()
	tm.CPU.Record()
	tm.User.Record()
	// reset timers
	tm.reset()

}

// reset reset the time fields of this time measure
func (tm *TimeMeasure) reset() {
	cpuTimeSys, cpuTimeUser := getRTime()
	tm.CPU = NewSingleMeasure(tm.name+"_system", cpuTimeSys)
	tm.User = NewSingleMeasure(tm.name+"_user", cpuTimeUser)
	tm.lastWallTime = time.Now()
}

// CounterIO is an interface that can be used to count how many bytes does an
// object have written and how many bytes does it have read. For example it is
// implemented by cothority/network/ Conn  + Host to know how many bytes a
// connection / Host has written /read
type CounterIO interface {
	// Rx returns the number of bytes read by this interface
	Rx() uint64
	// Tx returns the number of bytes transmitted / written by this interface
	Tx() uint64
}

// CounterIOMeasure is a struct that takes a CounterIO and can send the
// measurements to the monitor. Each time Record() is called, the measurements
// are put back to 0 (while the CounterIO still sends increased bytes number).
type CounterIOMeasure struct {
	name    string
	counter CounterIO
	baseTx  uint64
	baseRx  uint64
}

// NewCounterIOMeasure returns an CounterIOMeasure fresh. The base value are set
// to the current value of counter.Rx() and counter.Tx()
func NewCounterIOMeasure(name string, counter CounterIO) *CounterIOMeasure {
	return &CounterIOMeasure{
		name:    name,
		counter: counter,
		baseTx:  counter.Tx(),
		baseRx:  counter.Rx(),
	}
}

// Record send the actual number of bytes read and written (**name**_written &
// **name**_read) and reset the counters.
func (cm *CounterIOMeasure) Record() {
	// creates the read measure
	bRx := cm.counter.Rx()
	// TODO Later on, we might want to do a check on the conversion between
	// uint64 -> float64, as the MAX values are not the same.
	read := NewSingleMeasure(cm.name+"_rx", float64(bRx-cm.baseRx))
	// creates the  written measure
	bTx := cm.counter.Tx()
	written := NewSingleMeasure(cm.name+"_tx", float64(bTx-cm.baseTx))

	// send them both
	read.Record()
	written.Record()

	// reset counters
	cm.baseRx = bRx
	cm.baseTx = bTx
}

// CounterDockerMeasure is a struct that takes a docker-id and measures
// system and user time.
type DockerMeasure struct {
	name   string
	docker []string
	sys    []float64
	user   []float64
}

// NewDockerMeasure returns an CounterIOMeasure fresh. The base value are set
// to the current value of counter.Rx() and counter.Tx()
func NewDockerMeasure(name string, docker []string) *DockerMeasure {
	return &DockerMeasure{
		name:   name,
		docker: docker,
	}
}

func (dm *DockerMeasure) Update() {
	if len(dm.docker) == 0 {
		// Search all docker images
	}
	for i, d := range dm.docker {
		cpuFile := "/sys/fs/cgroup/cpuacct/docker/" + d + "/cpuacct.stat"
		cpu, err := ioutil.ReadFile(cpuFile)
		log.LLvl3("Getting docker-stats of", nil, cpuFile, cpu)
		if err == nil {
			times := strings.Split(string(cpu), "\n")
			user := strings.TrimPrefix(times[0], "user ")
			sys := strings.TrimPrefix(times[1], "system ")
			dm.user[i], err = strconv.ParseFloat(user, 64)
			if err != nil {
				log.Error(err)
			}
			dm.sys[i], err = strconv.ParseFloat(sys, 64)
			if err != nil {
				log.Error(err)
			}
			log.LLvl3("Stats are", dm.user[i], dm.sys[i])
		}
	}
}

// Record send the actual number of bytes read and written (**name**_written &
// **name**_read) and reset the counters.
func (dm *DockerMeasure) Record() {
	for i := range dm.docker {
		dn := dm.name + "_"
		NewSingleMeasure(dn+"sys", dm.sys[i]).Record()
		NewSingleMeasure(dn+"user", dm.user[i]).Record()
	}
}

// Send transmits the given struct over the network.
func send(v interface{}) error {
	if encoder == nil {
		return fmt.Errorf("Monitor's sink connection not initalized. Can not send any measures")
	}
	if !enabled {
		return nil
	}
	// For a large number of clients (˜10'000), the connection phase
	// can take some time. This is a linear backoff to enable connection
	// even when there are a lot of request:
	var ok bool
	var err error
	for wait := 500; wait < 1000; wait += 100 {
		if err = encoder.Encode(v); err == nil {
			ok = true
			break
		}
		log.Lvl1("Couldn't send to monitor-sink:", err)
		time.Sleep(time.Duration(wait) * time.Millisecond)
		continue
	}
	if !ok {
		return errors.New("Could not send any measures")
	}
	return nil
}

// EndAndCleanup sends a message to end the logging and closes the connection
func EndAndCleanup() {
	if err := send(NewSingleMeasure("end", 0)); err != nil {
		log.Error("Error while sending 'end' message:", err)
	}
	if err := connection.Close(); err != nil {
		// at least tell that we could not close the connection:
		log.Error("Could not close connecttion:", err)
	}
	encoder = nil
}

// Returns the difference of the given system- and user-time.
func getDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := getRTime()
	return nowSys - tSys, nowUsr - tUsr
}

// EnableMeasure will actually allow the sending of the measures if given true.
// Otherwise all measures won't be sent at all.
func EnableMeasure(b bool) {
	if b {
		log.Lvl3("Monitor: Measure enabled")
	} else {
		log.Lvl3("Monitor: Measure disabled")
	}
	enabled = b
}
