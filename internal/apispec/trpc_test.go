package apispec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTRPC_BasicRouter(t *testing.T) {
	t.Parallel()
	src := []byte(`
import { router, publicProcedure } from './trpc';
import { z } from 'zod';

export const userRouter = router({
  getById: publicProcedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => {
      return { id: input.id, name: 'X' };
    }),

  list: publicProcedure
    .query(async () => {
      return [];
    }),

  create: publicProcedure
    .input(z.object({ name: z.string(), email: z.string() }))
    .mutation(async ({ input }) => {
      return { id: '1', ...input };
    }),
});
`)
	c := ParseTRPC(src, "user-router.ts")
	if c == nil {
		t.Fatal("ParseTRPC returned nil")
	}
	if c.Kind != ContractTRPC {
		t.Errorf("kind = %q", c.Kind)
	}
	if len(c.Operations) != 3 {
		t.Fatalf("ops = %d, want 3: %+v", len(c.Operations), c.Operations)
	}

	byID := map[string]Operation{}
	for _, op := range c.Operations {
		byID[op.OperationID] = op
	}

	getById := byID["getById"]
	if getById.Method != "Query" {
		t.Errorf("getById method = %q, want Query", getById.Method)
	}
	if len(getById.FieldsWrite) != 1 || getById.FieldsWrite[0] != "id" {
		t.Errorf("getById FieldsWrite = %v", getById.FieldsWrite)
	}

	list := byID["list"]
	if list.Method != "Query" {
		t.Errorf("list method = %q", list.Method)
	}
	if len(list.FieldsWrite) != 0 {
		t.Errorf("list FieldsWrite should be empty: %v", list.FieldsWrite)
	}

	create := byID["create"]
	if create.Method != "Mutation" {
		t.Errorf("create method = %q, want Mutation", create.Method)
	}
	if len(create.FieldsWrite) != 2 {
		t.Errorf("create FieldsWrite = %v, want 2 (name, email)", create.FieldsWrite)
	}
}

func TestParseTRPC_NestedRouters(t *testing.T) {
	t.Parallel()
	src := []byte(`
import { router, publicProcedure } from './trpc';

export const appRouter = router({
  users: router({
    list: publicProcedure.query(async () => []),
    getById: publicProcedure.query(async () => null),
  }),
  posts: router({
    list: publicProcedure.query(async () => []),
  }),
});
`)
	c := ParseTRPC(src, "app.ts")
	if c == nil {
		t.Fatal("nil contract")
	}
	if len(c.Operations) != 3 {
		t.Fatalf("ops = %d, want 3 (users.list, users.getById, posts.list): %+v", len(c.Operations), c.Operations)
	}

	want := map[string]bool{
		"users.list":    true,
		"users.getById": true,
		"posts.list":    true,
	}
	for _, op := range c.Operations {
		if !want[op.OperationID] {
			t.Errorf("unexpected op id %q", op.OperationID)
		}
	}
}

func TestParseTRPC_NoTRPCImport(t *testing.T) {
	t.Parallel()
	src := []byte(`
import express from 'express';

const router = (cfg: any) => cfg;

export const myRouter = router({
  get: { fn: () => {} },
});
`)
	c := ParseTRPC(src, "fake.ts")
	if c != nil {
		t.Errorf("expected nil for non-tRPC file, got %+v", c)
	}
}

func TestParseTRPC_FindIntegration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "server"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "server", "router.ts"), []byte(`
import { router, publicProcedure } from '@trpc/server';
import { z } from 'zod';

export const userRouter = router({
  getById: publicProcedure.input(z.object({ id: z.string() })).query(async () => null),
});
`), 0o644)
	// Non-tRPC TS file should not appear.
	_ = os.WriteFile(filepath.Join(root, "server", "app.ts"), []byte(`
import express from 'express';
export const app = express();
`), 0o644)

	contracts, err := Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("contracts = %d, want 1: %+v", len(contracts), contracts)
	}
	if contracts[0].Kind != ContractTRPC {
		t.Errorf("kind = %q", contracts[0].Kind)
	}
}
