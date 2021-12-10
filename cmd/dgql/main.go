package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	dfuse "github.com/streamingfast/client-go"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var cmd = &cobra.Command{
	Use:   "dgql <endpoint> <file> [<variables>]",
	Short: "Query/stream dfuse GraphQL over gRPC interface.",
	Args:  cobra.RangeArgs(2, 3),
	Long: cobraDescription(`
		Query/stream dfuse GraphQL over gRPC interface at given <endpoint> using the following
		<file> and <variables> (if present).

		The scripts expect that DFUSE_API_TOKEN environment variable is set to a valid dfuse API
		token value.

		The script reads the file in argument, must be a valid GraphQL document, turns it into
		a proper GraphQL over gRPC query, add to this query object the <variables> argument (as-is,
		no transformation is done, so it must be valid JSON) and send the query to the server"

		If the response has '.data' field, the script extracts the content from it and returns
		it as valid JSON to the caller. Otherwise, the response is returned as-is if the '.data'
		field cannot be determined.

		If the '-r' (raw) option, the output of 'grpcurl' is returned without any transformation
		like described above. Currently, it's required to use this for stream that are never
		ending (or to get immediate feedback of a longer to complete stream).
	`),
	Example: cobraExamples(
		`dgql testnet.eos.dfuse.io:443 stream_transactions.graphql '{"query":"something:true"}'`,
		`dgql testnet.eos.dfuse.io:443 stream_transactions.graphql variables_file.json`,
	),
	SilenceErrors: false,
	SilenceUsage:  true,
	RunE:          dgqlE,
}

var flagAPIKey *string
var flagAuthURL *string
var flagInsecure *bool
var flagPlainText *bool
var flagRaw *bool

var zlog = zap.NewNop()
var tracer = logging.ApplicationLogger("dgql", "github.com/streamingfast/client-go/cmd/dgql", &zlog)

func main() {
	flagAPIKey = cmd.PersistentFlags().StringP("api-key", "a", "", "The dfuse API key to use to connect to the endpoint, if empty, checks enviornment variable DFUSE_API_KEY, if it's also empty, assumes the endpoint is not authenticated")
	flagAuthURL = cmd.PersistentFlags().String("auth-url", "", "The authentication URL server to use for convert the API key into an API token")
	flagInsecure = cmd.PersistentFlags().BoolP("insecure", "i", false, "Insecure gRPC TLS connection when connecting to a local endpoint (it skips certification validation)")
	flagPlainText = cmd.PersistentFlags().BoolP("plain-text", "p", false, "Plain-text gRPC connection (i.e. no TLS) when connecting to a local endpoint")
	flagRaw = cmd.PersistentFlags().BoolP("raw", "r", false, "Output GraphQL response as JSON untouched meaning you do get the 'data' and 'errors' fields and 'data' contains a string containing a JSON value")

	cmd.Execute()
}

func dgqlE(cmd *cobra.Command, args []string) error {
	config := newConfig(cmd, args)
	zlog.Info("performing graphql operation", zap.Reflect("config", config))

	options := []dfuse.ClientOption{
		dfuse.WithLogger(zlog),
	}

	if config.PlainText {
		options = append(options, dfuse.WithPlainText())
	}

	if config.Insecure {
		options = append(options, dfuse.WithInsecure())
	}

	if config.APIKey == "" {
		options = append(options, dfuse.WithoutAuthentication())
	}

	if config.AuthURL != "" {
		options = append(options, dfuse.WithAuthURL(config.AuthURL))
	}

	client, err := dfuse.NewClient(config.Endpoint, config.APIKey, options...)
	cli.NoError(err, "unable to create dfuse client")

	var variables dfuse.GraphQLVariables
	if config.Variables != "" {
		content := []byte(config.Variables)
		if cli.FileExists(config.Variables) {
			content, err = ioutil.ReadFile(config.Variables)
			cli.NoError(err, "unable to read variables file %q", config.Variables)
		}

		err := json.Unmarshal(content, &variables)
		if err != nil {
			return fmt.Errorf("unable to unmarshal variables: %w", err)
		}
	}

	stream, err := client.GraphQLSubscription(cmd.Context(), config.Document, variables)
	if err != nil {
		return fmt.Errorf("unable to open dfuse GraphQL over gRPC stream: %w", err)
	}

	for {
		message, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				zlog.Debug("stream completed")
				return nil
			}

			return fmt.Errorf("an error occurred while streaming results: %w", err)
		}

		if *flagRaw {
			out, err := json.Marshal(message)
			if err != nil {
				return fmt.Errorf("unable to marshal message to JSON: %w", err)
			}

			fmt.Println(string(out))
		} else {
			if len(message.Errors) > 0 {
				errOut, err := json.Marshal(message.Errors)
				if err != nil {
					return fmt.Errorf("unable to marshal errors to JSON: %w", err)
				}

				fmt.Fprintln(os.Stderr, string(errOut))
			}

			out, err := json.Marshal(json.RawMessage(message.Data))
			if err != nil {
				return fmt.Errorf("unable to marshal data to JSON: %w", err)
			}

			fmt.Println(string(out))
		}
	}
}

func readGraphQLDocument(cmd *cobra.Command, filename string) string {
	from := ""
	var reader io.Reader
	var err error

	if filename == "-" {
		from = "standard input"

		fi, err := os.Stdin.Stat()
		noError(err, "unable to stat stdin")
		ensure((fi.Mode()&os.ModeCharDevice) != 0, "request document to be read from stdin but it's not readable")

		reader = os.Stdin
	} else if fileExists(filename) {
		from = fmt.Sprintf("filename %q", filename)
		reader, err = os.Open(filename)
		noError(err, "unable to open file %q", filename)
		defer reader.(*os.File).Close()
	} else {
		from = "inline document"
		// We assume it's directly a GraphQL document
		reader = bytes.NewBufferString(filename)
	}

	document, err := ioutil.ReadAll(reader)
	noError(err, "unable to read GraphQL document from %s", from)

	return string(document)
}

type config struct {
	APIKey    string
	AuthURL   string
	File      string
	Variables string
	Endpoint  string
	Insecure  bool
	PlainText bool
	Document  string
}

var isMaybeJSONRegex = regexp.MustCompile("(\\{|,|\"|\\})")
var isLocalhostRegex = regexp.MustCompile("localhost(:[0-9]{2,5})?")
var hasPortSuffixRegex = regexp.MustCompile(":[0-9]{2,5}$")

func newConfig(cmd *cobra.Command, args []string) *config {
	out := &config{}

	if len(args) == 2 {
		out.Endpoint = args[0]
		out.File = args[1]
	} else if len(args) == 3 {
		out.Endpoint = args[0]
		out.File = args[1]
		out.Variables = args[2]
	} else {
		panic(fmt.Errorf("this should have been caught at flag processing, unexpected flag count %d", len(args)))
	}

	out.APIKey = *flagAPIKey
	if out.APIKey == "" {
		out.APIKey = os.Getenv("DFUSE_API_KEY")
	}

	out.AuthURL = *flagAuthURL
	if out.AuthURL == "" {
		out.AuthURL = os.Getenv("DFUSE_AUTH_URL")
	}

	ensureArgument(cmd, out.Endpoint != "", "The endpoint value must be specified")

	out.Insecure = *flagInsecure
	out.PlainText = *flagPlainText

	if !cmd.Flags().Changed("insecure") && !cmd.Flags().Changed("plain-text") {
		if strings.Contains(out.Endpoint, "*") {
			out.Insecure = true
			out.Endpoint = strings.ReplaceAll(out.Endpoint, "*", "")
		} else if isLocalhostRegex.MatchString(out.Endpoint) {
			out.PlainText = true
		}
	}

	out.Document = readGraphQLDocument(cmd, out.File)
	return out
}

func ensureArgument(cmd *cobra.Command, condition bool, message string, args ...interface{}) {
	if !condition {
		fmt.Printf("invalid arguments: "+message+"\n", args...)
		fmt.Println()

		cmd.Help()
		os.Exit(1)
	}
}

func ensure(condition bool, message string, args ...interface{}) {
	if !condition {
		noError(fmt.Errorf(message, args...), "invalid arguments")
	}
}

func noError(err error, message string, args ...interface{}) {
	if err != nil {
		quit(message+": "+err.Error(), args...)
	}
}

func quit(message string, args ...interface{}) {
	fmt.Printf(message+"\n", args...)
	os.Exit(1)
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func cobraDescription(in string) string {
	return dedent.Dedent(strings.Trim(in, "\n"))
}

func cobraExamples(in ...string) string {
	for i, line := range in {
		in[i] = "  " + line
	}

	return strings.Join(in, "\n")
}
