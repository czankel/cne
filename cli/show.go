package cli

import (
	"github.com/spf13/cobra"

	"github.com/czankel/cne/config"
)

var showCmd = &cobra.Command{
	Use:     "show",
	Short:   "Show resources",
	Aliases: []string{"g"},
	Args:    cobra.MinimumNArgs(1),
}

var showConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the environment configuration",
	Long: `Show the configuration for the environment in the current directory or globally
for all environments of the current user.
By default, this command returns the configuration derived from all
configuration files. The system option returns only the syste-wide
configuration and the user option the configuration for the user.`,
	RunE: showConfigRunE,
	Args: cobra.RangeArgs(0, 1),
}

var showSystemConfig bool
var showUserConfig bool

func showConfigRunE(cmd *cobra.Command, args []string) error {

	var conf *config.Config

	if showUserConfig == showSystemConfig {
		conf = config.Load()
	} else if showSystemConfig {
		conf = config.LoadSystemConfig()
	} else {
		conf = config.LoadUserConfig()
	}

	if len(args) == 0 {
		printStruct("Configuration", "Value", conf)
	} else {
		name := args[0]
		path, val, err := conf.Get(name)
		if err != nil {
			return err
		}

		printList([]struct {
			Configuration string
			Value         string
		}{{path, val}})
	}

	return nil
}

func init() {
	rootCmd.AddCommand(showCmd)
	showCmd.AddCommand(showConfigCmd)
	showConfigCmd.Flags().BoolVarP(
		&showSystemConfig, "system", "", false, "Show only system configurations")
	showConfigCmd.Flags().BoolVarP(
		&showUserConfig, "user", "", false, "Show only user configurations")
}
