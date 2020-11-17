package cli

import (
	"fmt"
	"os"

	"github.com/lithammer/dedent"
	"github.com/maelvls/users-grpc/pkg/cli/logutil"
	"github.com/mattn/go-isatty"
	"github.com/mgutz/ansi"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var verbose bool
var version Version
var client grpcClient

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "users-cli (list | search | create | get)",
	Short: "A nice CLI for querying users from the user-grpc microservice.",

	// https://github.com/spf13/cobra#prerun-and-postrun-hooks
	// This hook is also executed when subcommands are run.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		client = grpcClient{
			address: viper.GetString("address"),
		}
		logutil.Debugf("using address: %s", client.address)
		switch viper.GetString("color") {
		case "auto":
			ansi.DisableColors(!isatty.IsTerminal(os.Stdout.Fd()))
		case "always":
			ansi.DisableColors(false)
		case "never":
			ansi.DisableColors(true)
		default:
			logrus.Errorf("%s is not a valid value for --color; must be either 'auto', 'always' or 'never'", viper.GetString("color"))
			os.Exit(1)
		}
	},
	Long: dedent.Dedent(`
	For setting the address of the form HOST:PORT, you can
	* use the flag --address=:8000
	* or use the env var ADDRESS
	* or you can set 'address: localhost:8000' in $HOME/.users-cli.yml
	`),
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(v Version) {
	version = v
	if err := rootCmd.Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	logrus.SetFormatter(&logrus.TextFormatter{})
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.users-cli.yml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().String("address", ":8000", "'host:port' to bind to")
	rootCmd.PersistentFlags().String("color", "auto", "Supported are 'auto', 'always' and 'never'. In 'auto' mode, colors are enabled when stdout is a tty.")
	err := viper.BindPFlag("address", rootCmd.PersistentFlags().Lookup("address"))
	if err != nil {
		panic(err)
	}
	err = viper.BindPFlag("color", rootCmd.PersistentFlags().Lookup("color"))
	if err != nil {
		panic(err)
	}
}

type grpcClient struct {
	address string
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if verbose {
		logutil.EnableDebug = true
	}
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".users-cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logutil.Debugf("using config file: %v", viper.ConfigFileUsed())
	}
}
