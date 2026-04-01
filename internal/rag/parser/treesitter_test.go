package parser

import (
	"context"
	"strings"
	"testing"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

func TestTSParser_ParseFile(t *testing.T) {
	p := NewTSParser()
	ctx := context.Background()

	t.Run("Go file", func(t *testing.T) {
		content := `
package main
import "fmt"
type User struct { Name string }
func (u *User) Save() error { return nil }
func main() { fmt.Println("hello") }
`
		nodes, relations, err := p.ParseFile(ctx, "test.go", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		if len(nodes) == 0 {
			t.Errorf("expected nodes, got 0")
		}

		foundUser := false
		foundSave := false
		for _, n := range nodes {
			if n.Name == "User" {
				foundUser = true
			}
			if n.Name == "Save" {
				foundSave = true
			}
		}

		if !foundUser {
			t.Errorf("User struct not found")
		}
		if !foundSave {
			t.Errorf("Save method not found")
		}

		if len(relations) == 0 {
			t.Errorf("expected relations, got 0")
		}
	})

	t.Run("Python file", func(t *testing.T) {
		content := `
import os
def greet(name):
    print(f"Hello {name}")

class User:
    def __init__(self, name):
        self.name = name
    def save(self):
        pass

greet("world")
`
		nodes, _, err := p.ParseFile(ctx, "test.py", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		foundGreet := false
		foundUser := false
		for _, n := range nodes {
			if n.Name == "greet" {
				foundGreet = true
			}
			if n.Name == "User" {
				foundUser = true
			}
		}

		if !foundGreet {
			t.Errorf("greet function not found")
		}
		if !foundUser {
			t.Errorf("User class not found")
		}
	})

	t.Run("Rust file", func(t *testing.T) {
		content := `
use std::collections::HashMap;
fn main() {
    println!("Hello, world!");
}
struct User {
    name: String,
}
impl User {
    fn new(name: &str) -> Self {
        User { name: name.to_string() }
    }
}
`
		nodes, _, err := p.ParseFile(ctx, "test.rs", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		foundMain := false
		foundUser := false
		for _, n := range nodes {
			if n.Name == "main" {
				foundMain = true
			}
			if n.Name == "User" {
				foundUser = true
			}
		}

		if !foundMain {
			t.Errorf("main function not found")
		}
		if !foundUser {
			t.Errorf("User struct not found")
		}
	})

	t.Run("JavaScript file", func(t *testing.T) {
		content := `
import { useState } from 'react';
function greet(name) {
    console.log("Hello " + name);
}
class User {
    constructor(name) {
        this.name = name;
    }
    save() {
        return true;
    }
}
export default User;
`
		nodes, relations, err := p.ParseFile(ctx, "test.js", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		foundGreet := false
		foundUser := false
		for _, n := range nodes {
			if n.Name == "greet" {
				foundGreet = true
			}
			if n.Name == "User" {
				foundUser = true
			}
		}

		if !foundGreet {
			t.Errorf("greet function not found")
		}
		if !foundUser {
			t.Errorf("User class not found")
		}

		hasImport := false
		for _, r := range relations {
			if r.Type == "IMPORTS" {
				hasImport = true
				break
			}
		}
		if !hasImport {
			t.Errorf("import relation not found")
		}
	})

	t.Run("TypeScript file", func(t *testing.T) {
		content := `
import { Request, Response } from 'express';
interface User {
    id: number;
    name: string;
}
class UserService {
    async getUser(id: number): Promise<User> {
        return { id, name: "John" };
    }
}
export { User, UserService };
`
		nodes, relations, err := p.ParseFile(ctx, "test.ts", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		foundUserService := false
		foundInterface := false
		for _, n := range nodes {
			if n.Name == "UserService" {
				foundUserService = true
			}
			if n.Name == "User" && n.Kind == domain.KindInterface {
				foundInterface = true
			}
		}

		if !foundUserService {
			t.Errorf("UserService class not found")
		}
		if !foundInterface {
			t.Errorf("User interface not found")
		}

		hasImport := false
		for _, r := range relations {
			if r.Type == domain.RelImports {
				hasImport = true
				break
			}
		}
		if !hasImport {
			t.Errorf("import relation not found")
		}
	})

	t.Run("TSX file", func(t *testing.T) {
		content := `
import React from 'react';
interface Props {
    name: string;
}
const Greeting: React.FC<Props> = ({ name }) => {
    return <div>Hello {name}</div>;
};
export default Greeting;
`
		nodes, _, err := p.ParseFile(ctx, "test.tsx", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		foundProps := false
		for _, n := range nodes {
			if n.Name == "Props" {
				foundProps = true
				break
			}
		}

		if !foundProps {
			t.Errorf("Props interface not found")
		}
	})

	t.Run("Line number mapping for Go", func(t *testing.T) {
		content := `package main

func hello() string {
	return "world"
}

type User struct {
	Name string
}
`
		nodes, _, err := p.ParseFile(ctx, "test.go", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		var helloNode *domain.CodeNode
		var userNode *domain.CodeNode
		for i := range nodes {
			if nodes[i].Name == "hello" {
				helloNode = &nodes[i]
			}
			if nodes[i].Name == "User" {
				userNode = &nodes[i]
			}
		}

		if helloNode == nil {
			t.Fatal("hello function not found")
		}
		if helloNode.StartLine != 3 {
			t.Errorf("expected hello start line 3, got %d", helloNode.StartLine)
		}
		if helloNode.EndLine != 5 {
			t.Errorf("expected hello end line 5, got %d", helloNode.EndLine)
		}

		if userNode == nil {
			t.Fatal("User struct not found")
		}
		if userNode.StartLine != 7 {
			t.Errorf("expected User start line 7, got %d", userNode.StartLine)
		}
		if userNode.EndLine != 9 {
			t.Errorf("expected User end line 9, got %d", userNode.EndLine)
		}
	})

	t.Run("Line number mapping for TypeScript", func(t *testing.T) {
		content := `interface Config {
    apiKey: string;
    timeout: number;
}

class Service {
    constructor(private config: Config) {}
    
    async fetch(): Promise<void> {
        console.log(this.config);
    }
}
`
		nodes, _, err := p.ParseFile(ctx, "test.ts", []byte(content))
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		var configNode *domain.CodeNode
		var serviceNode *domain.CodeNode
		for i := range nodes {
			if nodes[i].Name == "Config" && nodes[i].Kind == domain.KindInterface {
				configNode = &nodes[i]
			}
			if nodes[i].Name == "Service" {
				serviceNode = &nodes[i]
			}
		}

		if configNode == nil {
			t.Fatal("Config interface not found")
		}
		if configNode.StartLine != 1 {
			t.Errorf("expected Config start line 1, got %d", configNode.StartLine)
		}

		if serviceNode == nil {
			t.Fatal("Service class not found")
		}
		if serviceNode.StartLine != 6 {
			t.Errorf("expected Service start line 6, got %d", serviceNode.StartLine)
		}
	})

	t.Run("Syntax error handling", func(t *testing.T) {
		content := `package main

func broken( {
	// Missing closing paren and brace
`

		result, err := p.ParseFileWithErrors(ctx, "broken.go", []byte(content))
		if err != nil {
			t.Fatalf("ParseFileWithErrors failed: %v", err)
		}

		if len(result.Errors) == 0 {
			t.Errorf("expected syntax errors, got 0")
		}

		if result.Errors[0].Line < 1 {
			t.Errorf("invalid error line number: %d", result.Errors[0].Line)
		}
	})

	t.Run("Supported extensions", func(t *testing.T) {
		extensions := p.SupportedExtensions()

		expected := []string{".go", ".py", ".rs", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"}
		for _, ext := range expected {
			found := false
			for _, e := range extensions {
				if e == ext {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected extension %s not found", ext)
			}
		}
	})

	t.Run("Unsupported extension returns error", func(t *testing.T) {
		_, _, err := p.ParseFile(ctx, "test.unknown", []byte("content"))
		if err == nil {
			t.Error("expected error for unsupported extension")
		}
		if !strings.Contains(err.Error(), "unsupported language") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
