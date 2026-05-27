package main

import "github.com/spf13/cobra"

var serverlessCmd = &cobra.Command{
	Use:   "serverless",
	Short: "Manage RunPod serverless endpoints",
}

func init() {
	serverlessCmd.AddCommand(serverlessUpCmd)
	serverlessCmd.AddCommand(serverlessDownCmd)
	serverlessCmd.AddCommand(serverlessDestroyCmd)
}
