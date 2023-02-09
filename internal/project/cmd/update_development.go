package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/takenet/deckard/internal/project"
)

const pattern = "^\\d+\\.\\d+\\.\\d+$"
const filePath = "internal/project/project.go"

var versionRegex, _ = regexp.Compile(pattern)

// This executable runs against version.go to upgrade to the next development version.
// If version.go contains the version 1.2.0, this command will modify the file to 1.2.1-SNAPSHOT.
func main() {
	ctx := context.Background()

	currentVersion := project.Version

	if strings.Contains(currentVersion, "-SNAPSHOT") {
		fmt.Println("Version should not contains '-SNAPSHOT'.")
		os.Exit(1)
	}

	if !versionRegex.MatchString(currentVersion) {
		fmt.Printf("Invalid version format. It should match with %s.\n", pattern)
		os.Exit(1)
	}

	versions := strings.Split(currentVersion, ".")
	versionPatch, err := strconv.Atoi(versions[2])

	if err != nil {
		logErrorAndExit(ctx, err)
	}

	newVersion := versions[0] + "." + versions[1] + "." + strconv.Itoa(versionPatch+1) + "-SNAPSHOT"

	fmt.Printf("New version: %s. Writing to %s.\n", newVersion, filePath)

	content, err := ioutil.ReadFile(filePath)

	if err != nil {
		logErrorAndExit(ctx, err)
	}

	result := strings.Replace(string(content), "Version = \""+currentVersion+"\"", "Version = \""+newVersion+"\"", 1)

	writeErr := ioutil.WriteFile(filePath, []byte(result), 0)
	if writeErr != nil {
		logErrorAndExit(ctx, writeErr)
	}

	fmt.Println("Version file updated.")
}

func logErrorAndExit(ctx context.Context, err error) {
	fmt.Printf("Error updating version.go to new version: %v.\n", err.Error())
	os.Exit(1)
}
