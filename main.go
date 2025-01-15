package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/seebs/gogetopt"
)

var srcPool *redis.Pool
var destPool *redis.Pool
var wgSrc sync.WaitGroup
var wgDest sync.WaitGroup
var start time.Time
var sentElems int
var stopkey string
var stopflag bool

const pidfile = "/opt/lock/aws2gcpUrlShortnerbridge_v1.pid"

type rstrings struct {
	rlen int
	rs   []string
}

func init() {
	catchSignals(pidfile)
	createPidFile(pidfile)
	stopkey = "stopaws2gcpUrlShortnerbridge_v1"
	stopflag = false
}

// Monitor go routine just runs in the background and logs the progress
func monitor(src string, qname string) {
	conn, _ := redis.Dial("tcp", src)
	conn.Do("del", stopkey)
	secs := 1
	res, err := redis.Int64(conn.Do("llen", qname))
	log.Printf("Length of queue %s is %d  Err=<%s>\n", qname, res, err)
	i := 0
	for {
		_, err = redis.Strings(conn.Do("BLPOP", stopkey, secs))
		if err == nil {
			stopflag = true
			log.Printf("EXIT: Returning from monitor subroutine Got exit signal\n")
			return
		}
		res, _ = redis.Int64(conn.Do("llen", qname))
		if res == 0 {
			// Do not fill logs with just queue=0 messages , printing only once in 10 times
			i++
			if i > 10 {
				i = 0
				log.Printf("No queue on server queuename=%s", qname)
			}
		} else {
			log.Printf("Length of queue %s is %d Speed is %f\n",
				qname, res, (float64(sentElems) / float64(secs)))
			i = 0
			sentElems = 0
		}
	}
}

func (r *rstrings) appendString(s string) int {
	r.rs = append(r.rs, s)
	r.rlen++
	return r.rlen
}

func newPool(server string, maxConn int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     maxConn,
		IdleTimeout: 240 * time.Second,
		Wait:        true,
		MaxActive:   2 * maxConn,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}
}

func destWorker(wid int, key string, c chan rstrings, stop chan bool) {
	conn := destPool.Get()
	defer wgDest.Done()
	defer conn.Close()
	_, err := conn.Do("PING")
	if err != nil {
		log.Printf("Could not connect to destination server %s\n", err)
		os.Exit(1)
	}

	defer log.Printf("Ending Dest worker %d\n\n", wid)
	log.Printf("Starting worker number %d\n", wid)
	//log.Printf("Adding elements to redis", strings.Join(s, ", "))
	for {
		select {
		case s := <-c:
			attempt := 1
			for {
				_, err := conn.Do("RPUSH", redis.Args{}.Add(key).AddFlat(s.rs)...)
				if err == nil {
					sentElems = sentElems + s.rlen

					break
				}

				log.Printf("Attempt %d Could not rpush to destination server %s\n", attempt, err)
				attempt++
				if attempt > 5 {
					log.Printf("FATAL: Too many retries for redis rpush .. Giving up \n")
					os.Exit(1)
				}
				time.Sleep(time.Second * time.Duration(attempt))
				conn = destPool.Get()

			}

		case <-stop:
			log.Printf("EXIT: Dest Worker id %d Got stop signal\n", wid)
			return

		}

	}

}

func srcWorker(wid int, key string, c chan rstrings, stop chan bool) {
	conn := srcPool.Get()
	defer wgSrc.Done()
	defer conn.Close()
	_, err := conn.Do("PING")
	if err != nil {
		log.Printf("Could not connect to src server %s\n", err)
		os.Exit(1)
	}
	log.Printf("Starting src worker number %d\n", wid)
	s := rstrings{}

	for {
		if stopflag {
			if s.rlen > 0 {
				c <- s
				s = rstrings{}
			}
			log.Printf("EXIT: Source worker exiting  \n")
			return
		}
		reply, err := redis.Strings(conn.Do("BLPOP", key, 5))
		if err == nil {
			if len(reply[1]) > 0 {
				s.appendString(reply[1])
				//log.Printf("Sending job %s\n\n", strings.Join(s.rs, ", "))
				c <- s
				s = rstrings{}
			}
		} else if err == redis.ErrNil {
			if s.rlen > 0 {
				c <- s
				s = rstrings{}
			}
		} else {
			log.Printf("ERROR = %s \n", err)
			stopflag = true
			break
		}

	}
	if s.rlen > 0 {
		c <- s
	}
}

func inputOpts() (cfg map[string]string) {
	type optg struct {
		sname  string
		lname  string
		dvalue string
	}

	// Sample call ./bridge -c 10 -s aredis.netcore.co.in:6379 -k PR1_PN_PUSHAMP -t PR1_PN_PUSHAMP

	invals := []optg{
		{"c", "concurrency", "10"},
		{"s", "src", "127.0.0.1:6379"},
		{"d", "dest", "127.0.0.1:6379"},
		{"k", "srckey", ""},
		{"t", "destkey", ""},
		{"n", "pname", "goRedisBridge"},
	}
	opts, _, _ := gogetopt.GetOpt(os.Args[1:], "s:d:k:t:c:n:")
	cfg = make(map[string]string)
	for _, r := range invals {
		if opts[r.sname] != nil {
			cfg[r.lname] = opts[r.sname].Value
		} else {
			cfg[r.lname] = r.dvalue
		}
	}
	return
}

func main() {
	cfg := inputOpts()
	openLog(syslog.LOG_INFO|syslog.LOG_LOCAL4, cfg["pname"])
	maxConn, _ := strconv.Atoi(cfg["concurrency"])
	log.Printf("Importing keys from %s to %s Concurrency=%d\n", cfg["src"], cfg["dest"], maxConn)

	srcPool = newPool(cfg["src"], maxConn)
	destPool = newPool(cfg["dest"], maxConn)

	conn := srcPool.Get()
	conn.Do("DEL", stopkey)

	jobs := make(chan rstrings)
	stop := make(chan bool)
	for w := 0; w < maxConn; w++ {
		go srcWorker(w, cfg["srckey"], jobs, stop)
		wgSrc.Add(1)
		go destWorker(w, cfg["destkey"], jobs, stop)
		wgDest.Add(1)
	}
	go monitor(cfg["src"], cfg["srckey"])
	wgSrc.Wait()
	log.Printf("Sending signal to dest child processes to exit\n")
	for w := 0; w < maxConn; w++ {
		stop <- true
	}
	wgDest.Wait()
	log.Printf("All Done bye\n\n")

	exitProg("All done", pidfile)

}

func createPidFile(pidfile string) {
	err := ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		panic(err)
	}
}

// OpenLog function opens a syslog handle and sets it to log
func openLog(syslogto syslog.Priority, pname string) {
	logwriter, e := syslog.New(syslogto, pname)
	if e == nil {
		log.SetOutput(logwriter)
	}
	log.SetFlags(0)
	log.Println("Starting goserver")
}

// CatchSignals just catched all INT TERM etc signals and exits after removing pidfile
func catchSignals(pidfile string) {

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		for sig := range signalChan {
			log.Printf("CATCHSIGNALS Got signal %v ", sig)
			stopflag = true

		}

	}()

}

// ExitProg Function is for removing pidfile on exit
func exitProg(msg string, pidfile string) {
	log.Printf("Exiting pushAmpBridge %s", msg)
	os.Remove(pidfile)
	os.Exit(0)
}
