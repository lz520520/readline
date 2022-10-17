package main

import (
	"fmt"

	"github.com/desertbit/readline"
)

func main() {
	cfg := &readline.Config{
		Prompt: "readline-remote: ",
	}
	handleFunc := func(rl *readline.Instance) {
		for {
			line, err := rl.Readline()
			if err != nil {
				break
			}
			fmt.Fprintln(rl.Stdout(), "receive:"+line)
		}
	}
	err := readline.ListenRemote("tcp", ":12344", cfg, handleFunc, nil, nil)
	if err != nil {
		println(err.Error())
	}
}
