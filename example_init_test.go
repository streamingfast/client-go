package dfuse_test

import (
	"fmt"
	"io/ioutil"

	"github.com/dfuse-io/logging"
)

func init() {
	logging.TestingOverride()
}

func graphqlDocumentFromFile(file string) string {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		panic(fmt.Errorf("graphql document from file: %w", err))
	}

	return string(content)
}
