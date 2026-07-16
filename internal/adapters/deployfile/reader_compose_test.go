package deployfile

import "testing"

const composeFixture = `
services:
  web:
    image: myco/web:1.4
    ports:
      - "8080:80"
      - 9090
    depends_on:
      - api
      - db
  api:
    image: myco/api:2.0
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres:16
`

func TestParseCompose_ServicesSortedByName(t *testing.T) {
	artifacts, err := ParseCompose("compose.yaml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("len(artifacts) = %d, want 3", len(artifacts))
	}
	names := []string{artifacts[0].Name, artifacts[1].Name, artifacts[2].Name}
	want := []string{"api", "db", "web"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("artifacts[%d].Name = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestParseCompose_FieldsAndKind(t *testing.T) {
	artifacts, err := ParseCompose("compose.yaml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	var web Artifact
	for _, a := range artifacts {
		if a.Name == "web" {
			web = a
		}
	}
	if web.Kind != "compose-service" {
		t.Errorf("Kind = %q, want %q", web.Kind, "compose-service")
	}
	if web.Image != "myco/web:1.4" {
		t.Errorf("Image = %q, want %q", web.Image, "myco/web:1.4")
	}
	if len(web.Ports) != 2 || web.Ports[0] != "8080:80" || web.Ports[1] != "9090" {
		t.Errorf("Ports = %v, want [8080:80 9090]", web.Ports)
	}
	if len(web.DependsOn) != 2 || web.DependsOn[0] != "api" || web.DependsOn[1] != "db" {
		t.Errorf("DependsOn = %v, want [api db]", web.DependsOn)
	}
	if web.File != "compose.yaml" {
		t.Errorf("File = %q, want %q", web.File, "compose.yaml")
	}
	if web.Line == 0 {
		t.Error("Line = 0, want a real (>0) G7 line pointer")
	}
}

func TestParseCompose_DependsOnMapForm(t *testing.T) {
	artifacts, err := ParseCompose("compose.yaml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	var api Artifact
	for _, a := range artifacts {
		if a.Name == "api" {
			api = a
		}
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "db" {
		t.Errorf("DependsOn (map form) = %v, want [db]", api.DependsOn)
	}
}

func TestParseCompose_NoServicesKey_ReturnsNilNotError(t *testing.T) {
	artifacts, err := ParseCompose("compose.yaml", []byte("version: \"3\"\n"))
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if artifacts != nil {
		t.Errorf("artifacts = %v, want nil", artifacts)
	}
}

func TestParseCompose_MalformedYAML_ReturnsError(t *testing.T) {
	_, err := ParseCompose("compose.yaml", []byte("services: [this is not a map\n"))
	if err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
}
