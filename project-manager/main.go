package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

func repoNameFromURL(url string) string {
	base := path.Base(url)                  // np. "example.git"
	return strings.TrimSuffix(base, ".git") // np. "example"
}

func ReadRepositoriesFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %s", err)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	return lines, scanner.Err()
}

func main() {
	repoPath := flag.String("path", "./repo-paths", "Provide a path for repository names file")
	destPath := flag.String("destination", "../", "Provide a destination path for cloned repositories")

	flag.Parse()

	lines, err := ReadRepositoriesFromFile(*repoPath)
	if err != nil {
		log.Fatalf("can't read repositories names from file: %s", err)
	}

	options := lines

	var selected []string

	prompt := &survey.MultiSelect{
		Message: "Select repositories to clone (space = select, enter = approve):",
		Options: options,
	}

	err = survey.AskOne(prompt, &selected)
	if err != nil {
		fmt.Println("error occured:", err)
		os.Exit(1)
	}

	for _, option := range selected {
		repoFolder := path.Join(*destPath, repoNameFromURL(option))

		cmd := exec.Command("git", "clone", option, repoFolder)
		cmd.Stdout = os.Stdout 
		cmd.Stderr = os.Stderr 

		if err = cmd.Run(); err != nil {
			fmt.Printf("can't exec command for repo: %s %s\n", option, err)
			continue
		}
	}
}
