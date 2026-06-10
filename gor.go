// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
package main

import (
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	httppptof "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func init() {
	var defaultServeMux http.ServeMux
	http.DefaultServeMux = &defaultServeMux

	http.HandleFunc("/debug/vars", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "{\n")
		first := true
		expvar.Do(func(kv expvar.KeyValue) {
			if kv.Key == "memstats" || kv.Key == "cmdline" {
				return
			}

			if !first {
				fmt.Fprintf(w, ",\n")
			}
			first = false
			fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
		})
		fmt.Fprintf(w, "\n}\n")
	})

	http.HandleFunc("/debug/pprof/", httppptof.Index)
	http.HandleFunc("/debug/pprof/cmdline", httppptof.Cmdline)
	http.HandleFunc("/debug/pprof/profile", httppptof.Profile)
	http.HandleFunc("/debug/pprof/symbol", httppptof.Symbol)
	http.HandleFunc("/debug/pprof/trace", httppptof.Trace)
}

func loggingMiddleware(addr string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			_, err := http.Get("http://" + addr)
			log.Println(err)
		}

		rb, _ := httputil.DumpRequest(r, false)
		log.Println(string(rb))
		next.ServeHTTP(w, r)
	})
}

// filterKnownFlags filters out unknown flags to prevent help display
func filterKnownFlags(args []string) []string {
	var filteredArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Skip non-flag arguments
		if !strings.HasPrefix(arg, "-") {
			filteredArgs = append(filteredArgs, arg)
			continue
		}

		// Handle special flags like --help, -h that should always be passed through
		flagName := strings.TrimLeft(arg, "-")
		if flagName == "help" || flagName == "h" {
			filteredArgs = append(filteredArgs, arg)
			continue
		}

		// Handle flags with = (e.g., --flag=value)
		if idx := strings.Index(arg, "="); idx != -1 {
			flagName = strings.TrimLeft(arg[:idx], "-")
		}

		// Check if this flag is defined
		if flag.CommandLine.Lookup(flagName) != nil {
			filteredArgs = append(filteredArgs, arg)

			// If flag doesn't contain = and next arg is not a flag, include it as value
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				filteredArgs = append(filteredArgs, args[i])
			}
		}
		// Unknown flags are silently ignored
	}

	return filteredArgs
}

func main() {
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	}

	args := os.Args[1:]
	var plugins *InOutPlugins
	if len(args) > 0 && args[0] == "file-server" {
		if len(args) != 2 {
			log.Fatal("You should specify port and IP (optional) for the file server. Example: `gor file-server :80`")
		}
		dir, _ := os.Getwd()

		Debug(0, "Started example file server for current directory on address ", args[1])

		log.Fatal(http.ListenAndServe(args[1], loggingMiddleware(args[1], http.FileServer(http.Dir(dir)))))
	} else {
		// Parse flags but ignore unknown flags instead of showing help
		flag.CommandLine.Parse(filterKnownFlags(os.Args[1:]))
		checkSettings()

		// Initialize log file if log directory is specified
		if err := initLogFile(); err != nil {
			log.Fatalf("Failed to initialize log file: %v", err)
		}

		plugins = NewPlugins()
	}

	log.Printf("[PPID %d and PID %d] Version:%s\n", os.Getppid(), os.Getpid(), VERSION)

	if len(plugins.Inputs) == 0 || len(plugins.Outputs) == 0 {
		log.Fatal("Required at least 1 input and 1 output")
	}

	if *memprofile != "" {
		profileMEM(*memprofile)
	}

	if *cpuprofile != "" {
		profileCPU(*cpuprofile)
	}

	if Settings.Pprof != "" {
		go func() {
			log.Println(http.ListenAndServe(Settings.Pprof, nil))
		}()
	}

	closeCh := make(chan int)
	emitter := NewEmitter()
	go emitter.Start(plugins, Settings.Middleware)
	if Settings.ExitAfter > 0 {
		log.Printf("Running gor for a duration of %s\n", Settings.ExitAfter)

		time.AfterFunc(Settings.ExitAfter, func() {
			log.Printf("gor run timeout %s\n", Settings.ExitAfter)
			close(closeCh)
		})
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	exit := 0
	select {
	case <-c:
		exit = 1
	case <-closeCh:
		exit = 0
	}
	emitter.Close()

	// Close log file before exit
	closeLogFile()

	os.Exit(exit)
}

func profileCPU(cpuprofile string) {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		time.AfterFunc(30*time.Second, func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}
}

func profileMEM(memprofile string) {
	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Fatal(err)
		}
		time.AfterFunc(30*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}
}
