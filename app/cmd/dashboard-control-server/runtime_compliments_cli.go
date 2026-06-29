package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) runComplimentsCLI(args []string) int {
	cmd := "list"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}
	payload := a.complimentsPayload()
	items := jsonutil.List(payload["messages"])
	save := func() int {
		payload["messages"] = items
		payload["version"] = 4
		if err := fileio.WriteJSON(a.complimentsPath(), payload); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	switch cmd {
	case "list", "":
		if len(items) == 0 {
			fmt.Println("No messages yet.")
			return 0
		}
		for i, raw := range items {
			m := jsonutil.Map(raw)
			extra := ""
			if d := jsonutil.StringValue(m["date"]); d != "" {
				extra += " [on " + d + "]"
			}
			if w := jsonutil.Int(m["weight"], 1); w != 1 {
				extra += fmt.Sprintf(" [weight %d]", w)
			}
			fmt.Printf("%3d) %s%s\n", i+1, fmt.Sprint(m["text"]), extra)
		}
		return 0
	case "search":
		q := strings.ToLower(strings.Join(args, " "))
		for i, raw := range items {
			m := jsonutil.Map(raw)
			if strings.Contains(strings.ToLower(fmt.Sprint(m["text"])), q) {
				fmt.Printf("%3d) %s\n", i+1, fmt.Sprint(m["text"]))
			}
		}
		return 0
	case "add":
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "usage: compliments.sh add TEXT [--weight N] [--date MM-DD]")
			return 64
		}
		body := map[string]any{"text": args[0], "origin": "custom"}
		for i := 1; i < len(args); i++ {
			if args[i] == "--weight" && i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					body["weight"] = n
				}
				i++
			} else if args[i] == "--date" && i+1 < len(args) {
				if args[i+1] != "" {
					body["date"] = args[i+1]
				}
				i++
			}
		}
		item, err := cleanCompliment(body, map[string]any{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		item["id"] = nextNumericID(items)
		items = append(items, item)
		if rc := save(); rc != 0 {
			return rc
		}
		fmt.Println("message added")
		return 0
	case "remove":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: compliments.sh remove N")
			return 64
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 || n > len(items) {
			fmt.Fprintln(os.Stderr, "invalid item number")
			return 1
		}
		items = append(items[:n-1], items[n:]...)
		if rc := save(); rc != 0 {
			return rc
		}
		fmt.Println("message removed")
		return 0
	case "edit":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: compliments.sh edit N TEXT [--weight N] [--date MM-DD]")
			return 64
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 || n > len(items) {
			fmt.Fprintln(os.Stderr, "invalid item number")
			return 1
		}
		existing := jsonutil.Map(items[n-1])
		body := map[string]any{"text": args[1]}
		for i := 2; i < len(args); i++ {
			if args[i] == "--weight" && i+1 < len(args) {
				if x, err := strconv.Atoi(args[i+1]); err == nil {
					body["weight"] = x
				}
				i++
			} else if args[i] == "--date" && i+1 < len(args) {
				if args[i+1] != "" {
					body["date"] = args[i+1]
				}
				i++
			}
		}
		item, err := cleanCompliment(body, existing)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		item["id"] = jsonutil.Int(existing["id"], n)
		items[n-1] = item
		if rc := save(); rc != 0 {
			return rc
		}
		fmt.Println("message updated")
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: compliments.sh [list|search|add|edit|remove]")
		return 64
	}
}
