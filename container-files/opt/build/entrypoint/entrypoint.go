package main

import (
	"encoding/json"
	"github.com/flosch/pongo2/v5"
	"github.com/pkg/errors"
	"log"
	"os"
	"os/exec"
	"strings"
)

const caddyFileSrc = "/usr/templates/Caddyfile.j2"
const caddyFilePath = "/etc/caddy/Caddyfile"

const wpFileSrc = "/usr/templates/wp-rules.conf.j2"
const wpFilePath = "/etc/caddy/rules/wordpress/rules.conf"

func main() {
	usesOwnCaddyfile := getEnv("OWN_CADDYFILE", false).(bool)
	usesWordpressRules := getEnv("ENABLE_RULE_WORDPRESS", false).(bool)
	renderer := renderer{}
	renderer.buildPongoContext()
	if err := renderer.populateUpstreams(); err != nil {
		log.Fatal(err)
	}

	if usesWordpressRules {
		if err := renderer.renderFile(wpFileSrc, wpFilePath); err != nil {
			log.Fatalf("Cannot render Wordpress Ruleset: %v", err)
		}
	}

	if !usesOwnCaddyfile {
		if err := renderer.renderFile(caddyFileSrc, caddyFilePath); err != nil {
			log.Fatalf("Cannot render Caddyfile: %v", err)
		}
	}

	if len(os.Args) < 3 {
		log.Fatalf("Need to provide at least 2 arguments e.g. /usr/bin/caddy run -pidfile /tmp/pid -config /etc/caddy/Caddyfile")
	}

	// run caddy server
	caddy := exec.Command(os.Args[1], os.Args[2:]...)
	caddy.Stdout = os.Stdout
	caddy.Stdin = os.Stdin
	caddy.Stderr = os.Stderr

	if err := caddy.Run(); err != nil {
		log.Fatalf("Process exited with error: %v", err)
	}
}

type renderer struct {
	upstreams []Upstream
	ctx       pongo2.Context
}

func (r *renderer) renderFile(sourcePath string, targetPath string) error {
	if err := r.populateUpstreams(); err != nil {
		return err
	}

	template := pongo2.Must(pongo2.FromFile(caddyFileSrc))
	out, err := template.Execute(r.ctx)
	if err != nil {
		return errors.Wrapf(err, "Cannot parse file '%v'", sourcePath)
	}

	if err := os.WriteFile(targetPath, []byte(out), os.FileMode(0755)); err != nil {
		return errors.Wrapf(err, "Cannot write into file %v", targetPath)
	}

	return nil
}

func (r *renderer) buildPongoContext() {
	ctx := pongo2.Context{}

	// environment variable = regular variable
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		ctx[pair[0]] = getEnv(pair[0], "")
	}

	// all collected upstreams are internally objects
	ctx["upstreams"] = r.upstreams

	r.ctx = ctx
}

func (r *renderer) populateUpstreams() error {
	// collect upstream configuration
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if strings.Contains(pair[0], "UPSTREAM_") {
			upstream := &Upstream{}

			err := json.Unmarshal([]byte(pair[1]), upstream)
			if err != nil {
				return errors.Wrapf(err, "Cannot parse upstream configuration from: %v", env)
			}

			r.upstreams = append(r.upstreams, *upstream)
		}
	}
	return nil
}

func getEnv(name string, defaultValue interface{}) interface{} {
	value, exists := os.LookupEnv(name)
	if !exists {
		return defaultValue
	}
	if value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "YES" || value == "Y" || value == "y" {
		return true
	}
	if value == "0" || value == "false" || value == "FALSE" || value == "no" || value == "NO" || value == "N" || value == "n" {
		return false
	}
	return value
}

type Upstream struct {
	PassTo   string `json:"pass_to"`
	Hostname string `json:"hostname"`
}
