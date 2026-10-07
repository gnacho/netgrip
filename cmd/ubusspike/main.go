// Command ubusspike measures native ubus client latency against forked
// `ubus call` CLI invocations. It is a throwaway benchmark for issue #461.
//
// Usage:
//
//	ubusspike -mode native -iters 10 'system.board' '{}'
//	ubusspike -mode cli -iters 10 system board '{}'
//
// Modes:
//   - native: one Dial per iteration (Lookup + Invoke over the unix socket)
//   - cli: exec "ubus call <obj> <method> <json>" per iteration
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/ubusconn"
)

func main() {
	mode := flag.String("mode", "native", "native|cli")
	iters := flag.Int("iters", 10, "iterations")
	socket := flag.String("socket", "", "ubus socket path (native, default /var/run/ubus/ubus.sock)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ubusspike -mode native|cli -iters N <object> <method> <json-args>")
		os.Exit(2)
	}
	object, method, rawArgs := args[0], args[1], args[2]

	var jsonArgs map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &jsonArgs); err != nil {
		fmt.Fprintf(os.Stderr, "bad json args: %v\n", err)
		os.Exit(2)
	}

	var durs []time.Duration
	for i := 0; i < *iters; i++ {
		start := time.Now()
		var err error
		switch *mode {
		case "native":
			err = nativeCall(*socket, object, method, jsonArgs)
		case "cli":
			err = cliCall(object, method, rawArgs)
		default:
			fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
			os.Exit(2)
		}
		durs = append(durs, time.Since(start))
		if err != nil {
			fmt.Fprintf(os.Stderr, "iter %d: %v\n", i, err)
			os.Exit(1)
		}
	}

	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	med := durs[len(durs)/2]
	if len(durs)%2 == 0 {
		med = (durs[len(durs)/2-1] + durs[len(durs)/2]) / 2
	}
	fmt.Printf("mode=%s object=%s method=%s iters=%d min=%s med=%s max=%s\n",
		*mode, object, method, *iters, durs[0], med, durs[len(durs)-1])
}

func nativeCall(socket, object, method string, args map[string]any) error {
	c, err := ubusconn.Dial(socket)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Call(object, method, args)
	return err
}

func cliCall(object, method, rawArgs string) error {
	out, err := exec.Command("ubus", "call", object, method, rawArgs).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("ubus call: %v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return err
	}
	// Validate the output is well-formed JSON, like the panel would.
	var v map[string]any
	return json.Unmarshal(out, &v)
}
