package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/realnedsanders/symphony/src/internal/author"
	"github.com/realnedsanders/symphony/src/internal/httpapi"
	"github.com/realnedsanders/symphony/src/internal/loop"
	"github.com/realnedsanders/symphony/src/internal/model"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "loop":
		err = runLoop(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "author":
		err = runAuthor(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: symphony loop|serve|author [flags]")
}

func runLoop(args []string) error {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	fixture := fs.String("fixture", "testdata/loop", "fixture workspace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text, err := loop.Run(*fixture)
	if err != nil {
		return err
	}
	fmt.Print(text)
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	workspace := fs.String("workspace", "workspace", "git workspace")
	web := fs.String("web", "web", "web UI directory")
	addr := fs.String("addr", "127.0.0.1:0", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	handler, err := httpapi.New(*workspace, *web)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	fmt.Printf("listening %s\n", ln.Addr())
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return server.Serve(ln)
}

func runAuthor(args []string) error {
	fs := flag.NewFlagSet("author", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "git workspace")
	part := fs.String("part", "", "part name")
	owner := fs.String("owner", "", "part owner")
	iface := fs.String("interface", "", "interface name")
	shape := fs.String("shape", "v1", "interface shape")
	configKey := fs.String("config-key", "", "configuration key")
	configValue := fs.String("config-value", "", "configuration value")
	sensorName := fs.String("sensor", "", "sensor contract name")
	observes := fs.String("observes", "", "what the sensor observes")
	aim := fs.String("aim", "", "how the sensor is aimed")
	outside := fs.String("outside", "", "what lies outside coverage")
	decision := fs.String("decision", "", "decision name")
	about := fs.String("about", "", "element the decision concerns")
	choice := fs.String("choice", "", "chosen path")
	alternative := fs.String("alternative", "", "rejected alternative")
	rationale := fs.String("rationale", "", "rationale")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workspace == "" || *part == "" {
		return fmt.Errorf("author: -workspace and -part are required")
	}
	goal := model.Goal{Parts: []model.Part{{Name: *part, Def: "Component"}}}
	if *owner != "" {
		goal.Owners = append(goal.Owners, model.Ownership{Part: *part, Owner: *owner})
	}
	if *configKey != "" {
		goal.Configurations = append(goal.Configurations, model.Configuration{Part: *part, Key: *configKey, Value: *configValue})
	}
	if *iface != "" {
		goal.Interfaces = append(goal.Interfaces, model.Interface{Name: *iface, Shape: *shape})
		goal.Places = append(goal.Places, model.Place{Part: *part, Port: "input", Interface: *iface})
	}
	if *sensorName != "" {
		goal.Sensors = append(goal.Sensors, model.SensorContract{
			Name: *sensorName, Observes: *observes, Aim: *aim, Outside: *outside,
		})
	}
	if *decision != "" {
		goal.Decisions = append(goal.Decisions, model.Decision{
			Name: *decision, About: *about, Choice: *choice, Alternative: *alternative, Rationale: *rationale, Status: "rejected",
		})
	}
	commit, err := author.Apply(context.Background(), *workspace, goal)
	if err != nil {
		return err
	}
	fmt.Println(commit)
	return nil
}
