package main // import "go.sbr.pm/nr"

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
)

const (
	conf    = ".config/nr"
	cmdTmpl = `#!/usr/bin/env bash
# Generated by nr {{ .profile }}
{{ .args }} $@
`
)

var (
	force = flag.Bool("force", false, "Should command be created even if already present in path ?")
)

type alias struct {
	Command string `json:"cmd"`
	Package string `json:"pkg"`
	Channel string `json:"chan"`
}

func main() {
	flag.Parse()
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) != 2 {
		log.Fatal("expected only one argument (name of the alias file)")
	}
	profile := os.Args[1]
	fmt.Println("> nr generate", profile)
	aliases, err := readAliases(filepath.Join(home, conf, profile))
	if err != nil {
		log.Fatal(err)
	}
	if err := cleanGenerated(home, profile); err != nil {
		log.Fatal(err)
	}
	if err := generate(home, profile, aliases, *force); err != nil {
		log.Fatal(err)
	}
}

func readAliases(path string) ([]alias, error) {
	aliases := []alias{}
	config, err := ioutil.ReadFile(path)
	if err != nil {
		return aliases, err
	}
	if err := json.NewDecoder(strings.NewReader(string(config))).Decode(&aliases); err != nil {
		return aliases, err
	}
	return aliases, nil
}

func cleanGenerated(home, profile string) error {
	bins := filepath.Join(home, "bin")
	files, err := ioutil.ReadDir(bins)
	if err != nil {
		return err
	}
	removes := []string{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		f, err := os.Open(filepath.Join(bins, file.Name()))
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "# Generated by nr "+profile) {
				removes = append(removes, file.Name())
				break
			}
		}
		f.Close()
	}
	for _, file := range removes {
		if err := os.Remove(filepath.Join(bins, file)); err != nil {
			return err
		}
	}
	return nil
}

func generate(home, profile string, aliases []alias, force bool) error {
	t := template.Must(template.New("cmd").Parse(cmdTmpl))
	for _, a := range aliases {
		if _, err := os.Stat(filepath.Join(home, ".nix-profile/bin/", a.Command)); os.IsNotExist(err) || force {
			// create command
			pkg := a.Package
			if pkg == "" {
				pkg = a.Command
			}
			channel := a.Channel
			if channel == "" {
				channel = "nixpkgs"
			}
			c := []string{"nix", "run"}
			if channel != "nixpkgs" {
				c = append(c, "-f", "~/.config/nixpkgs/channels.nix")
			}
			c = append(c, channel+"."+pkg, "-c", a.Command)
			f, err := os.Create(filepath.Join(home, "bin", a.Command))
			if err != nil {
				return err
			}
			if err := t.Execute(f, map[string]interface{}{
				"profile": profile,
				"args":    strings.Join(c, " "),
			}); err != nil {
				f.Close()
				return err
			}
			f.Close()
			if err := os.Chmod(filepath.Join(home, "bin", a.Command), 0777); err != nil {
				return err
			}
		} else {
			fmt.Printf("> %s already exists\n", a.Command)
		}
	}
	return nil
}
