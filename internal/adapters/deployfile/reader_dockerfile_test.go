package deployfile

import "testing"

func TestParseDockerfile_FromAndExpose(t *testing.T) {
	content := []byte(`FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/app ./cmd/app

FROM alpine:3.19
COPY --from=build /out/app /usr/local/bin/app
EXPOSE 8080
EXPOSE 9090/udp
`)
	a, ok := ParseDockerfile("Dockerfile", content)
	if !ok {
		t.Fatal("expected ok=true for a Dockerfile with a FROM line")
	}
	if a.Kind != "dockerfile" {
		t.Errorf("Kind = %q, want %q", a.Kind, "dockerfile")
	}
	// Multi-stage build: the LAST FROM is what actually ships (earlier
	// stages are build-time only) — "what ships" is this view's whole v1
	// premise.
	if a.Image != "alpine:3.19" {
		t.Errorf("Image = %q, want final FROM's base image %q", a.Image, "alpine:3.19")
	}
	if a.Line != 6 {
		t.Errorf("Line = %d, want 6 (final FROM)", a.Line)
	}
	wantPorts := []string{"8080", "9090/udp"}
	if len(a.Ports) != len(wantPorts) {
		t.Fatalf("Ports = %v, want %v", a.Ports, wantPorts)
	}
	for i, p := range wantPorts {
		if a.Ports[i] != p {
			t.Errorf("Ports[%d] = %q, want %q", i, a.Ports[i], p)
		}
	}
}

func TestParseDockerfile_NoFromLine_ReturnsFalse(t *testing.T) {
	_, ok := ParseDockerfile("Dockerfile", []byte("# just a comment\n"))
	if ok {
		t.Fatal("expected ok=false when no FROM line is present")
	}
}

func TestParseDockerfile_Empty_ReturnsFalse(t *testing.T) {
	_, ok := ParseDockerfile("Dockerfile", []byte(""))
	if ok {
		t.Fatal("expected ok=false for empty content")
	}
}
