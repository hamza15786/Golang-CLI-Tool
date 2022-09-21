/*
Doppler Substitute Command line tool

Run the following commands : go build doppler.go
cp doppler /usr/local/bin

Usage: doppler substitute --format "variable expression format" --source "input file path" --destination "dest directory path"

Example: doppler substitute --format dollar-curly --source /Users/hamza/files --destination /Users/hamza/go-project/wow

*/

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//fetch value from the JSON format string for the provided secret
func extractValue(body string, key string) string {
	keystr := "\"" + key + "\":[^,;\\]}]*"
	r, _ := regexp.Compile(keystr)
	match := r.FindString(body)
	keyValMatch := strings.Split(match, ":")
	return strings.ReplaceAll(keyValMatch[1], "\"", "")
}

func main() {

	// sub command
	substituteCommand := flag.NewFlagSet("substitute", flag.ExitOnError)

	//flags/arguments for substitute command
	subFormatPtr := substituteCommand.String("format", "dollar-curly", "Desired secret var format")
	inputPathPtr := substituteCommand.String("source", "", "input file/files path")
	outputPathPtr := substituteCommand.String("destination", "", "file destination directory")

	if len(os.Args) < 2 {
		fmt.Println("subcommand is required") // validate that we are getting correct number of arguments
		os.Exit(1)
	}

	substituteCommand.Parse(os.Args[2:])
	substituteCommand.Parse(os.Args[3:])
	substituteCommand.Parse(os.Args[4:])

	if *subFormatPtr == "" || *inputPathPtr == "" || *outputPathPtr == "" {
		fmt.Println("Usage: doppler --format \"variable expression format\" --source \"input file path\" --destination \"dest directory path\"")
		os.Exit(1)
	}

	url := "https://api.doppler.com/v3/configs/config/secrets/download?project=testing&config=dev&format=json&include_dynamic_secrets=false"

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")
	//req.Header.Add("authorization", "Basic ") // (ADD SERVICE TOKEN) fetching secrets directly from Doppler API using Service Token

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	secret := (string(body))

	path := *outputPathPtr
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) { // Check if output directory exists, if not create it
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Println(err)
		}
	}

	var result map[string]interface{} // convert JSON secrets to map

	json.Unmarshal([]byte(secret), &result)

	var files []string
	root := *inputPathPtr

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			fmt.Println(err)
			return nil
		}

		if !info.IsDir() { // recursilvely creating an array of all files required to be substituted

			files = append(files, path)
		}
		return nil

	})

	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files { // iterating through all files

		sourceFile, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}

		newFile, err := os.Create(*outputPathPtr + "/" + filepath.Base(file)) // creating substitution files to be placed at destination
		if err != nil {
			log.Fatal(err)

		}

		io.Copy(newFile, sourceFile)

		for k := range result { // going through all available secret key values to check for potential matches within each file

			input, err := ioutil.ReadFile(newFile.Name())
			if err != nil {
				fmt.Println(err)

				os.Exit(1)
			}
			var new string
			if *subFormatPtr == "dollar" {
				new = "$" + k
			}
			if *subFormatPtr == "dollar-curly" { //  accounting for the different possible variable expressions
				new = "${" + k + "}"
			}
			if *subFormatPtr == "handlebars" {
				new = "{{" + k + "}}"
			}
			if *subFormatPtr == "dollar-handlebars" {
				new = "${{" + k + "}}"
			}

			output := bytes.Replace(input, []byte(new), []byte(extractValue(secret, k)), -1) //fetching secrets from JSON string and replacing variable expressions when key value is found
			if err = ioutil.WriteFile(newFile.Name(), output, 0666); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		}

	}

}
