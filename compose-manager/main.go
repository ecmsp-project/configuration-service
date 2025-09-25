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
	"github.com/manifoldco/promptui"
	"golang.org/x/exp/maps"
)

func getRepoNameFromURL(url string) string {
	base := path.Base(url)
	return strings.TrimSuffix(base, ".git")
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

func getRepoNameToPathMapping(repoPaths []string) map[string]string {
	repoMap := make(map[string]string)

	for _, repoPath := range repoPaths {
		repoMap[getRepoNameFromURL(repoPath)] = repoPath
	}

	return repoMap
}

type Result struct {
	repoMap            map[string]string
	currentService     string
	selectedCluster    []string

	shouldCloneMissing string
	shouldStartCurrentDb string

	destPath string
}

func PromptUI(repoMap map[string]string) *Result {
	validate := func(input string) error {
		_, exists := repoMap[input]
		if !exists {
			return fmt.Errorf("provided service name doesn't exists: %s", input)
		}
		return nil
	}

	aPrompt := promptui.Prompt{
		Label:    "Name of the service you are currently developing",
		Validate: validate,
	}
	currentServiceName, err := aPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return nil
	}

	validate = func(input string) error {
		if input != "yes" && input != "no" {
			return fmt.Errorf("supported values: [yes/no]")
		}
		return nil
	}
	bPrompt := promptui.Prompt{
		Label:    "Should clone missing repositories (in cluster) [yes/no]:",
		Default:  "no",
		Validate: validate,
	}
	shouldCloneMissing, err := bPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return nil
	}

	var selected []string
	cPrompt := &survey.MultiSelect{
		Message: "Select services to run in Docker cluster (except the one you are currently developing):",
		Options: maps.Keys(repoMap),
	}
	err = survey.AskOne(cPrompt, &selected)
	if err != nil {
		fmt.Println("error occured:", err)
		return nil
	}

	validate = func(input string) error {
		if input != "yes" && input != "no" {
			return fmt.Errorf("supported values: [yes/no]")
		}
		return nil
	}
	dPrompt := promptui.Prompt{
		Label:    "Should run DB (dev) in the service you are currently developing [yes/no]:",
		Default:  "no",
		Validate: validate,
	}
	shouldStartCurrentDb, err := dPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return nil
	}

	return &Result{
		repoMap:            repoMap,
		currentService:     currentServiceName,
		selectedCluster:    selected,

		shouldCloneMissing: shouldCloneMissing,
		shouldStartCurrentDb: shouldStartCurrentDb,
	}
}

func cloneMissingRepos(result *Result) {
	if result.shouldCloneMissing == "no" {
		return
	}

	for _, repoName := range result.selectedCluster {
		repoFolder := path.Join(result.destPath, repoName)

		cmd := exec.Command("git", "clone", result.repoMap[repoName], repoFolder)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("can't exec command for repo: %s %s\n", repoName, err)
			return
		}
	}
}

func getProfilesOptions(result *Result) string {
	profiles := "--profile kafka"

	for _, repoName := range result.selectedCluster {
		profiles = profiles + fmt.Sprintf(" --profile %s", repoName)
	}

	return profiles
}

func startCurrentDb(result *Result) {
	if result.shouldStartCurrentDb == "no" {
		return 
	}

	composeFilePath := path.Join(result.destPath, result.currentService)
	composeFilePath = composeFilePath + "/docker"

	service := fmt.Sprintf("%s-db-dev", result.currentService)

	cmd := exec.Command("docker", "compose", "-f", composeFilePath, "up", service)
	cmd.Stdout = os.Stdout 
	cmd.Stderr = os.Stderr 

	if err := cmd.Run(); err != nil {
		fmt.Printf("can't exec 'docker compose <profiles> up' command to start current db")
	}
}

func RunCluster(result *Result) {

	// Clone missing repositories
	cloneMissingRepos(result)

	// Create --profile string to apply in docker compose command
	profiles := getProfilesOptions()

	// Run internal db in current service 
	startCurrentDb(result)

	// Run Docker compose command 
	cmd := exec.Command("docker", "compose", profiles, "up")
	cmd.Stdout = os.Stdout 
	cmd.Stderr = os.Stderr 

	if err := cmd.Run(); err != nil {
		fmt.Printf("can't exec 'docker compose <profiles> up' command")
	}

}

func main() {
	repoPathsFile := flag.String("path", "./repo-paths.txt", "Provide a path for repository paths file")
	destPath := flag.String("destination", "../../", "Provide a destination path for cloned repositories")

	flag.Parse()

	repoPaths, err := ReadRepositoriesFromFile(*repoPathsFile)
	if err != nil {
		log.Fatalf("can't read repositories names from file: %s", err)
	}

	repoMap := getRepoNameToPathMapping(repoPaths)

	result := PromptUI(repoMap)
	result.destPath = *destPath

	RunCluster(result)
}
