/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License,
 * version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract 
 * DE-AC04-94AL85000 with Sandia Corporation, the U.S. Government retains 
 * certain rights in this software.
 */

/*
 * We were using the scanner but it was too much and too little: I could never get it to scan IP addresses correctly. 
 * So we just make the incoming string a stack and pop() things from it. 
 */

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Stack string

func (s *Stack) push(tos string) {
	*s = Stack(tos + " " + string(*s))
}


func (s *Stack) tos() (string) {
	stack := strings.SplitN(string(*s), " ", 2)
	return stack[0]
}

func (s *Stack) pop() (tos string) {
	stack := strings.SplitN(string(*s), " ", 2)
	if len(stack) > 1 {
		*s = Stack(stack[1])
	} else {
		*s = ""
	}
	return stack[0]
}

func (s *Stack) binop(op string) {
	op2,_ := strconv.Atoi(s.pop())
	op1,_ := strconv.Atoi(s.pop())
	var res int

	switch(op) {
		case "*":
			res = op1 * op2
		case "+":
			res = op1 + op2
		case "-":
			res = op1 - op2
		case "/":
			res = op1 / op2
		case "%":
			res = op1 % op2
	}
	s.push(strconv.Itoa(res))
	return
}

func (s *Stack) triop(op string) {
	op1 := s.pop()
	op2 := s.pop()
	op3 := s.pop()
	switch(op) {
		case "ifelse":
			op1val,_ := strconv.Atoi(op1)
			if op1val == 0 {
				s.push(op2)
			} else {
				s.push(op3)
			}
	}
	return
}


/*
 * stack machine for parsing simple language. 
 */
func forth(c string) (string){
	var stack Stack
	fmt.Printf("FORTH: START>>>>>>>>>>>>>>>>\n") 
	in := Stack(c)
	fmt.Printf("in: %v\n", string(in))
	/* we won't use full tokenization yet because we're not sure we need it */
	for len(string(in)) > 0 {
		command := in.pop()
		switch(command) {
		case "hostname":
			hostname, _ := os.Hostname()
			stack.push(hostname)
		case "base": 
			host := stack.pop()
			stack.push(strings.TrimLeft(host, "abcdefghijklmnopqrstuvwxyz -"))
		case "*":
			stack.binop( command)
		case "+":
			stack.binop( command)
		case "-":
			stack.binop( command)
		case "/":
			stack.binop( command)
		case "%":
			stack.binop( command)
		case "ifelse":
			stack.triop( command)
		case "dup":
			tos := stack.tos()
			stack.push(tos)
		case "roundup":
			rnd,_ := strconv.Atoi(stack.pop())
			v,_ := strconv.Atoi(stack.pop())
			v = ((v + rnd-1)/rnd)*rnd
			stack.push(strconv.Itoa(v))
		case "strcat":
			s1 := stack.pop()
			s2 := stack.pop()
			stack.push(s1 + s2)
		/* discard empty junk left by Split */
		default: 
			if len(command) > 0 {
				stack.push(command)
			}
		}
		fmt.Printf("Op: %v; Stack: %v\n", command, string(stack))
	}
	fmt.Printf("FORTH: END>>>>>>>>>>>>>>>>%v\n", stack.tos())
	return stack.pop()
}
