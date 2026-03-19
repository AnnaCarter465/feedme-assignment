package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"feedme-assignment/internal"
)

func main() {
	f, err := os.Create("scripts/result.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create result.txt:", err)
		os.Exit(1)
	}
	defer f.Close()

	out := io.MultiWriter(os.Stdout, f)

	log := func(format string, args ...any) {
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(out, "[%s] "+format+"\n", append([]any{ts}, args...)...)
	}

	c := internal.NewController(log)

	log("=== McDonald's Order Controller — Demo ===")

	// --- Step 1: create orders before any bots ---
	c.AddOrder(internal.Normal) // #1
	c.AddOrder(internal.VIP)    // #2 → jumps ahead of #1
	c.AddOrder(internal.Normal) // #3

	// --- Step 2: add two bots ---
	// Bot#1 picks VIP#2, Bot#2 picks Normal#1; Normal#3 waits
	c.AddBot()
	c.AddBot()

	// --- Wait past the 10-second processing window ---
	// At t≈10s: Bot#1 completes VIP#2, picks Normal#3
	//           Bot#2 completes Normal#1, goes IDLE
	time.Sleep(12 * time.Second)
	c.Status()

	// --- Step 3: new VIP order while Bot#2 is idle ---
	// Bot#2 picks it up immediately
	c.AddOrder(internal.VIP) // #4

	time.Sleep(3 * time.Second)

	// --- Step 4: remove Bot#2 while it is processing VIP#4 ---
	// VIP#4 returns to PENDING; Bot#1 will pick it up after finishing Normal#3
	c.RemoveBot()

	// Bot#1 finishes Normal#3 at t≈20s, then picks VIP#4, finishes at t≈30s
	time.Sleep(17 * time.Second)
	c.Status()

	// --- Step 5: remove Bot#1 (now IDLE) ---
	c.RemoveBot()

	c.FinalStatus()
}
