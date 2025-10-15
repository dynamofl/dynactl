package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dynamoai/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

// AddRegistryCommands registers registry related commands with the root command.
func AddRegistryCommands(rootCmd *cobra.Command) {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage OCI registry credentials",
		Long:  "Manage authentication credentials used when accessing OCI registries.",
	}

	loginCmd := &cobra.Command{
		Use:   "login <registry>",
		Short: "Store credentials for an OCI registry",
		Long:  "Store credentials for the given OCI registry so dynactl can authenticate without relying on external CLIs.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := args[0]
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			passwordStdin, _ := cmd.Flags().GetBool("password-stdin")
			identityToken, _ := cmd.Flags().GetString("identity-token")
			accessToken, _ := cmd.Flags().GetString("access-token")

			if passwordStdin && password != "" {
				return fmt.Errorf("--password and --password-stdin cannot be used together")
			}

			if passwordStdin {
				reader := bufio.NewReader(os.Stdin)
				data, err := reader.ReadBytes('\n')
				if err != nil && err.Error() != "EOF" {
					return fmt.Errorf("failed to read password from stdin: %w", err)
				}
				password = strings.TrimRight(string(data), "\r\n")
			}

			if password != "" && username == "" {
				return fmt.Errorf("--username is required when providing a password")
			}

			if password == "" && identityToken == "" && accessToken == "" {
				return fmt.Errorf("must supply either --password, --identity-token, or --access-token")
			}

			cred := utils.RegistryCredential{
				Username:      username,
				Password:      password,
				IdentityToken: identityToken,
				AccessToken:   accessToken,
			}

			if err := utils.SaveRegistryCredential(registry, cred); err != nil {
				return err
			}

			cmd.Printf("✅ Stored credentials for %s\n", registry)
			return nil
		},
	}

	loginCmd.Flags().StringP("username", "u", "", "Username for registry authentication")
	loginCmd.Flags().StringP("password", "p", "", "Password for registry authentication")
	loginCmd.Flags().Bool("password-stdin", false, "Read password from standard input")
	loginCmd.Flags().String("identity-token", "", "Identity (refresh) token for registry authentication")
	loginCmd.Flags().String("access-token", "", "Access token for registry authentication")

	registryCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(registryCmd)
}
