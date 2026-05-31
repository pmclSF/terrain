package apispec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProto_Basic(t *testing.T) {
	t.Parallel()
	src := `syntax = "proto3";

package billing.v1;

message GetInvoiceRequest {
  string invoice_id = 1;
  bool include_line_items = 2;
}

message GetInvoiceResponse {
  string invoice_id = 1;
  string customer_id = 2;
  int64 amount_cents = 3;
  repeated LineItem line_items = 4;
}

message LineItem {
  string description = 1;
  int64 amount_cents = 2;
}

service BillingService {
  rpc GetInvoice(GetInvoiceRequest) returns (GetInvoiceResponse);
}
`
	c := ParseProto(src)
	if c.Version != "proto3" {
		t.Errorf("version = %q", c.Version)
	}
	if len(c.Operations) != 1 {
		t.Fatalf("ops = %d, want 1", len(c.Operations))
	}
	op := c.Operations[0]
	if op.Method != "RPC" {
		t.Errorf("method = %q", op.Method)
	}
	if op.Path != "/BillingService/GetInvoice" {
		t.Errorf("path = %q", op.Path)
	}
	if op.OperationID != "BillingService.GetInvoice" {
		t.Errorf("op id = %q", op.OperationID)
	}
	if len(op.FieldsRead) != 4 {
		t.Errorf("FieldsRead = %v, want 4 (response fields)", op.FieldsRead)
	}
	if len(op.FieldsWrite) != 2 {
		t.Errorf("FieldsWrite = %v, want 2 (request fields)", op.FieldsWrite)
	}
}

func TestParseProto_StreamingFlavors(t *testing.T) {
	t.Parallel()
	src := `syntax = "proto3";

message Req { string id = 1; }
message Resp { string ok = 1; }

service Streamer {
  rpc UnaryCall(Req) returns (Resp);
  rpc ServerStream(Req) returns (stream Resp);
  rpc ClientStream(stream Req) returns (Resp);
  rpc BidiStream(stream Req) returns (stream Resp);
}
`
	c := ParseProto(src)
	if len(c.Operations) != 4 {
		t.Fatalf("ops = %d, want 4", len(c.Operations))
	}
	want := map[string]string{
		"UnaryCall":    "RPC",
		"ServerStream": "RPC-SERVER-STREAM",
		"ClientStream": "RPC-CLIENT-STREAM",
		"BidiStream":   "RPC-BIDI",
	}
	for _, op := range c.Operations {
		// op.OperationID = "Streamer.<method>"
		method := op.OperationID[len("Streamer."):]
		if op.Method != want[method] {
			t.Errorf("method %s = %q, want %q", method, op.Method, want[method])
		}
	}
}

func TestParseProto_NestedTypesNotConfused(t *testing.T) {
	t.Parallel()
	src := `syntax = "proto3";

message Outer {
  string outer_field = 1;
  message Inner {
    string inner_field = 1;
  }
  oneof choice {
    string choice_a = 2;
    int32 choice_b = 3;
  }
  enum Kind {
    UNKNOWN = 0;
    ACTIVE = 1;
  }
  Inner inner = 4;
}

service S {
  rpc Get(Outer) returns (Outer);
}
`
	c := ParseProto(src)
	if len(c.Operations) != 1 {
		t.Fatalf("ops = %d", len(c.Operations))
	}
	// FieldsWrite should be Outer's DIRECT fields: outer_field, inner.
	// choice_a, choice_b are inside oneof (a nested block); should be excluded
	// from the top-level direct-field enumeration.
	got := c.Operations[0].FieldsWrite
	if len(got) != 2 {
		t.Errorf("FieldsWrite = %v, want 2 (outer_field, inner — nested removed)", got)
	}
}

func TestParseProto_QualifiedTypes(t *testing.T) {
	t.Parallel()
	src := `syntax = "proto3";

import "google/protobuf/empty.proto";

message Empty {}

service Health {
  rpc Check(google.protobuf.Empty) returns (Empty);
}
`
	c := ParseProto(src)
	if len(c.Operations) != 1 {
		t.Fatalf("ops = %d", len(c.Operations))
	}
	// google.protobuf.Empty resolves to "Empty" in the local messages
	// map; since we don't declare it ourselves the FieldsWrite is empty.
	if len(c.Operations[0].FieldsWrite) != 0 {
		t.Errorf("FieldsWrite for unknown message = %v, want empty", c.Operations[0].FieldsWrite)
	}
}

func TestParseProtoFile_FindIntegration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "proto")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "billing.proto"), []byte(`syntax = "proto3";

message Req { string id = 1; }
message Resp { string ok = 1; }

service S { rpc Do(Req) returns (Resp); }
`), 0o644)

	contracts, err := Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("contracts = %d, want 1", len(contracts))
	}
	if contracts[0].Kind != ContractGRPC {
		t.Errorf("kind = %q", contracts[0].Kind)
	}
}
