package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	repoMap         map[string]string
	currentService  string
	selectedCluster []string

	shouldCloneMissing   string
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
		Label:    "Should clone missing repositories (in cluster) [yes/no]",
		Default:  "no",
		Validate: validate,
	}
	shouldCloneMissing, err := bPrompt.Run()
	if err != nil {
		log.Error().Msgf("Prompt failed %v\n", err)
		return nil
	}

	// TODO: print all services except the one the user is working on
	var selected []string
	cPrompt := &survey.MultiSelect{
		Message: "Select services to run in Docker cluster (except the one you are currently developing)",
		Options: maps.Keys(repoMap),
	}
	err = survey.AskOne(cPrompt, &selected)
	if err != nil {
		log.Error().Msgf("error occured: %v", err)
		return nil
	}

	validate = func(input string) error {
		if input != "yes" && input != "no" {
			return fmt.Errorf("supported values: [yes/no]")
		}
		return nil
	}
	dPrompt := promptui.Prompt{
		Label:    "Should run DB (dev) in the service you are currently developing [yes/no]",
		Default:  "no",
		Validate: validate,
	}
	shouldStartCurrentDb, err := dPrompt.Run()
	if err != nil {
		log.Error().Msgf("Prompt failed %v\n", err)
		return nil
	}

	return &Result{
		repoMap:         repoMap,
		currentService:  currentServiceName,
		selectedCluster: selected,

		shouldCloneMissing:   shouldCloneMissing,
		shouldStartCurrentDb: shouldStartCurrentDb,
	}
}

func cloneMissingRepos(result *Result) error {
	log.Info().Msg("cloning missing repositories")

	if result.shouldCloneMissing == "no" {
		// TODO: check if all repositories are present then. If not, print and return error
		return nil
	}

	for _, repoName := range result.selectedCluster {
		repoFolder := path.Join(result.destPath, repoName)

		_, err := os.Stat(repoFolder)
		if err == nil {
			log.Warn().Msgf("repository is already cloned: %s", repoName)
			continue 
		}

		cmd := exec.Command("git", "clone", result.repoMap[repoName], repoFolder)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			log.Error().Msgf("can't exec command for repo: %s %s\n", repoName, err)
			return err
		}
	}

	return nil
}

func getArgsOptions(result *Result) []string {

	args := []string{"compose"}

	for _, repoName := range result.selectedCluster {
		args = append(args, "--profile", repoName)
	}

	args = append(args, "--profile", "kafka")
	args = append(args, "up", "-d")

	return args
}

func startCurrentDb(result *Result) error {
	log.Info().Msgf("starting currently developed DB in %s", result.currentService)

	if result.shouldStartCurrentDb == "no" {
		return nil
	}

	composeFilePath := path.Join(result.destPath, result.currentService)
	composeFilePath = composeFilePath + "/docker/docker-compose.yml"

	service := fmt.Sprintf("%s-db-dev", result.currentService)

	cmd := exec.Command("docker", "compose", "-f", composeFilePath, "up", service, "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error().Msg("can't exec 'docker compose <profiles> up' command to start current db")
		return err
	}

	return nil 
}

func RunCluster(result *Result) {

	// Clone missing repositories
	err := cloneMissingRepos(result)
	if err != nil {
		return 
	}

	// Run internal db in current service
	err = startCurrentDb(result)
	if err != nil {
		return 
	}

	// Run Docker compose command
	log.Info().Msg("executing docker compose to setup cluster")

	// Create arguments to run cluster 
	args := getArgsOptions(result)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error().Msg("can't exec 'docker compose <profiles> up' command")
		return 
	}

}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	repoPathsFile := flag.String("path", "./repo-paths.txt", "Provide a path for repository paths file")
	destPath := flag.String("destination", "../../", "Provide a destination path for cloned repositories")

	flag.Parse()

	repoPaths, err := ReadRepositoriesFromFile(*repoPathsFile)
	if err != nil {
		log.Error().Msgf("can't read repositories names from file: %s", err)
	}

	repoMap := getRepoNameToPathMapping(repoPaths)

	result := PromptUI(repoMap)
	result.destPath = *destPath

	RunCluster(result)
}
