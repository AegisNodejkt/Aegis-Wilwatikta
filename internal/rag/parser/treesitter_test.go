package parser

import (
	"context"
	"testing"
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
}
