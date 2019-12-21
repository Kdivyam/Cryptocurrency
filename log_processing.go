package main

import (
	"bufio"
	"os"
)

var lines []string

func main() {

	lines = readLines("/home/divyamk2/distributed/mp2/transaction.txt")

}

func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
