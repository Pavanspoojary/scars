package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPythonSymbolHash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-py-ast-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pyFile := filepath.Join(tmpDir, "calculator.py")
	initialContent := `import math

def calculate_fee(amount, tier="standard"):
    # Calculate processing fee based on customer tier
    base = amount * 0.02
    if tier == "premium":
        return base * 0.8
    return base

class TaxCalculator:
    def compute(self, subtotal):
        return subtotal * 0.07

async def async_dispatch(task_id):
    pass
`
	if err := os.WriteFile(pyFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed writing python file: %v", err)
	}

	// 1. Extract function hash
	fnHash1, err := ExtractPythonSymbolHash(pyFile, "calculate_fee")
	if err != nil {
		t.Fatalf("failed extracting python function hash: %v", err)
	}
	if fnHash1 == "" {
		t.Fatal("expected non-empty hash for calculate_fee")
	}

	// 2. Extract class hash
	classHash1, err := ExtractPythonSymbolHash(pyFile, "TaxCalculator")
	if err != nil {
		t.Fatalf("failed extracting python class hash: %v", err)
	}
	if classHash1 == "" {
		t.Fatal("expected non-empty hash for TaxCalculator")
	}

	// 3. Extract async def hash
	asyncHash1, err := ExtractPythonSymbolHash(pyFile, "async_dispatch")
	if err != nil {
		t.Fatalf("failed extracting async def hash: %v", err)
	}
	if asyncHash1 == "" {
		t.Fatal("expected non-empty hash for async_dispatch")
	}

	// 4. Modify unrelated code and comments in file
	modifiedContent := `import math
import os # new unused import

def calculate_fee(amount, tier="standard"):
    # Modified comment inside function
    base = amount * 0.02
    if tier == "premium":
        return base * 0.8
    return base

class TaxCalculator:
    def compute(self, subtotal):
        return subtotal * 0.10 # modified tax rate

async def async_dispatch(task_id):
    pass
`
	if err := os.WriteFile(pyFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed writing modified python file: %v", err)
	}

	// calculate_fee AST/token body is unchanged (only comment and outside imports changed)
	fnHash2, err := ExtractPythonSymbolHash(pyFile, "calculate_fee")
	if err != nil {
		t.Fatalf("failed extracting python function hash after modification: %v", err)
	}
	if fnHash1 != fnHash2 {
		t.Errorf("expected identical hash for calculate_fee despite comment/import changes, got %s != %s", fnHash1, fnHash2)
	}

	// TaxCalculator was modified inside its body
	classHash2, err := ExtractPythonSymbolHash(pyFile, "TaxCalculator")
	if err != nil {
		t.Fatalf("failed extracting class hash: %v", err)
	}
	if classHash1 == classHash2 {
		t.Errorf("expected different hash for modified TaxCalculator, got identical %s", classHash1)
	}
}

func TestExtractTSSymbolHash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-ts-ast-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tsFile := filepath.Join(tmpDir, "service.ts")
	initialContent := `import { Request, Response } from 'express';

export async function authenticateUser(token: string): Promise<boolean> {
  // Validate token payload
  if (!token) {
    return false;
  }
  return token.startsWith("valid_");
}

export const processPayment = (amount: number): boolean => {
  return amount > 0;
};

export class SessionManager {
  private active: boolean = true;
  public isValid(): boolean {
    return this.active;
  }
}
`
	if err := os.WriteFile(tsFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed writing ts file: %v", err)
	}

	// 1. Extract function hash
	authHash1, err := ExtractTSSymbolHash(tsFile, "authenticateUser")
	if err != nil {
		t.Fatalf("failed extracting authenticateUser hash: %v", err)
	}
	if authHash1 == "" {
		t.Fatal("expected non-empty hash for authenticateUser")
	}

	// 2. Extract arrow const hash
	paymentHash1, err := ExtractTSSymbolHash(tsFile, "processPayment")
	if err != nil {
		t.Fatalf("failed extracting processPayment hash: %v", err)
	}
	if paymentHash1 == "" {
		t.Fatal("expected non-empty hash for processPayment")
	}

	// 3. Extract class hash
	classHash1, err := ExtractTSSymbolHash(tsFile, "SessionManager")
	if err != nil {
		t.Fatalf("failed extracting SessionManager hash: %v", err)
	}
	if classHash1 == "" {
		t.Fatal("expected non-empty hash for SessionManager")
	}

	// 4. Modify file with unrelated changes
	modifiedContent := `import { Request, Response } from 'express';
import { logger } from './logger'; // new import

export async function authenticateUser(token: string): Promise<boolean> {
  // Comment updated
  if (!token) {
    return false;
  }
  return token.startsWith("valid_");
}

export const processPayment = (amount: number): boolean => {
  return amount > 10; // modified threshold!
};

export class SessionManager {
  private active: boolean = true;
  public isValid(): boolean {
    return this.active;
  }
}
`
	if err := os.WriteFile(tsFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed writing modified ts file: %v", err)
	}

	authHash2, err := ExtractTSSymbolHash(tsFile, "authenticateUser")
	if err != nil {
		t.Fatalf("failed extracting authenticateUser hash: %v", err)
	}
	if authHash1 != authHash2 {
		t.Errorf("expected identical hash for authenticateUser despite comment changes, got %s != %s", authHash1, authHash2)
	}

	paymentHash2, err := ExtractTSSymbolHash(tsFile, "processPayment")
	if err != nil {
		t.Fatalf("failed extracting processPayment hash: %v", err)
	}
	if paymentHash1 == paymentHash2 {
		t.Errorf("expected different hash for modified processPayment, got identical %s", paymentHash1)
	}
}

func TestStoreFreshnessMultiLanguage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-store-freshness-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Create a Python file
	pyRel := "services/auth.py"
	pyFull := filepath.Join(tmpDir, pyRel)
	_ = os.MkdirAll(filepath.Dir(pyFull), 0755)
	initialPy := `def check_permissions(user_id):
    return user_id == "admin"

def helper_func():
    return True
`
	_ = os.WriteFile(pyFull, []byte(initialPy), 0644)

	// Create Scar anchoring check_permissions
	scar := &Scar{
		SchemaVersion: "1",
		ID:            "scar-py-01",
		Title:         "Admin check vulnerability",
		Lesson:        "Check roles not string",
		Lifecycle:     Lifecycle{State: LifecycleActive},
		Anchors: Anchors{
			Files:   []string{pyRel},
			Symbols: []string{"check_permissions"},
		},
	}

	// Record baseline
	if err := store.RecordBaseline(scar, "hash-cmd"); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}

	if len(scar.Baseline.SymbolHashes) == 0 || scar.Baseline.SymbolHashes["check_permissions"] == "" {
		t.Fatalf("expected recorded symbol hash for check_permissions, got %+v", scar.Baseline.SymbolHashes)
	}

	// Verify initial freshness is Current
	if fresh := store.ComputeFreshness(scar); fresh != FreshnessCurrent {
		t.Errorf("expected FreshnessCurrent, got %s", fresh)
	}

	// Edit helper_func (unanchored)
	modifiedPy := `def check_permissions(user_id):
    return user_id == "admin"

def helper_func():
    # Modified comment and return
    return False
`
	_ = os.WriteFile(pyFull, []byte(modifiedPy), 0644)

	// Even though file hash changed, symbol hash of check_permissions is identical!
	if fresh := store.ComputeFreshness(scar); fresh != FreshnessCurrent {
		t.Errorf("expected FreshnessCurrent when unanchored function changed, got %s", fresh)
	}

	// Now modify check_permissions itself
	brokenPy := `def check_permissions(user_id):
    return True # Security bypass!

def helper_func():
    return False
`
	_ = os.WriteFile(pyFull, []byte(brokenPy), 0644)

	if fresh := store.ComputeFreshness(scar); fresh != FreshnessChanged {
		t.Errorf("expected FreshnessChanged when anchored symbol changed, got %s", fresh)
	}
}
