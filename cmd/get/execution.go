package get

import (
	"context"
	"fmt"
	"github.com/disiqueira/gotree"
	"sort"
	"strconv"

	"github.com/flyteorg/flytectl/cmd/config"
	"github.com/flyteorg/flytectl/cmd/config/subcommand/execution"
	cmdCore "github.com/flyteorg/flytectl/cmd/core"
	"github.com/flyteorg/flytectl/pkg/printer"
	"github.com/flyteorg/flyteidl/gen/pb-go/flyteidl/admin"
	"github.com/flyteorg/flytestdlib/logger"
	"github.com/golang/protobuf/proto"
)

const (
	executionShort = "Gets execution resources"
	executionLong  = `
Retrieves all the executions within project and domain.(execution,executions can be used interchangeably in these commands)
::

 bin/flytectl get execution -p flytesnacks -d development

Retrieves execution by name within project and domain.

::

 bin/flytectl get execution -p flytesnacks -d development oeh94k9r2r

Retrieves all the executions with filters.
::
 
  bin/flytectl get execution -p flytesnacks -d development --filter.field-selector="execution.phase in (FAILED;SUCCEEDED),execution.duration<200" 

 
Retrieves all the execution with limit and sorting.
::
  
   bin/flytectl get execution -p flytesnacks -d development --filter.sort-by=created_at --filter.limit=1 --filter.asc
   

Retrieves all the execution within project and domain in yaml format

::

 bin/flytectl get execution -p flytesnacks -d development -o yaml

Retrieves all the execution within project and domain in json format.

::

 bin/flytectl get execution -p flytesnacks -d development -o json

Usage
`
)

var hundredChars = 100

var executionColumns = []printer.Column{
	{Header: "Name", JSONPath: "$.id.name"},
	{Header: "Launch Plan Name", JSONPath: "$.spec.launchPlan.name"},
	{Header: "Type", JSONPath: "$.spec.launchPlan.resourceType"},
	{Header: "Phase", JSONPath: "$.closure.phase"},
	{Header: "Started", JSONPath: "$.closure.startedAt"},
	{Header: "Elapsed Time", JSONPath: "$.closure.duration"},
	{Header: "Abort data (Trunc)", JSONPath: "$.closure.abortMetadata[\"cause\"]", TruncateTo: &hundredChars},
	{Header: "Error data (Trunc)", JSONPath: "$.closure.error[\"message\"]", TruncateTo: &hundredChars},
}


var nodeExecutionColumns = []printer.Column{
	{Header: "Name", JSONPath: "$.id.nodeId"},
	{Header: "Exec", JSONPath: "$.id.executionId.name"},
	{Header: "StartedAt", JSONPath: "$.closure.startedAt"},
	{Header: "Duration", JSONPath: "$.closure.duration"},
	{Header: "Started", JSONPath: "$.closure.startedAt"},
	{Header: "Phase", JSONPath: "$.closure.phase"},
}

func ExecutionToProtoMessages(l []*admin.Execution) []proto.Message {
	messages := make([]proto.Message, 0, len(l))
	for _, m := range l {
		messages = append(messages, m)
	}
	return messages
}

func NodeExecutionToProtoMessages(l []*admin.NodeExecution) []proto.Message {
	messages := make([]proto.Message, 0, len(l))
	for _, m := range l {
		messages = append(messages, m)
	}
	return messages
}

func getExecutionFunc(ctx context.Context, args []string, cmdCtx cmdCore.CommandContext) error {
	adminPrinter := printer.Printer{}
	var executions []*admin.Execution
	if len(args) > 0 {
		name := args[0]
		exec, err := cmdCtx.AdminFetcherExt().FetchExecution(ctx, name, config.GetConfig().Project, config.GetConfig().Domain)
		if err != nil {
			return err
		}
		executions = append(executions, exec)
		logger.Infof(ctx, "Retrieved %v executions", len(executions))

		if execution.DefaultConfig.Details {
			// Fetching Node execution details
			nExecDetails, nodeExecToTaskExec, err := getNodeExecDetailsWithTasks(ctx, config.GetConfig().Project, config.GetConfig().Domain, name, cmdCtx)
			if err != nil {
				return err
			}
			if !execution.DefaultConfig.DefaultView {
				// Print tree view
				printNodeDetailsTreeView(nExecDetails, nodeExecToTaskExec)
				return nil
			}
			return adminPrinter.Print(config.GetConfig().MustOutputFormat(), nodeExecutionColumns,
				NodeExecutionToProtoMessages(nExecDetails)...)
		}
		return adminPrinter.Print(config.GetConfig().MustOutputFormat(), executionColumns,
			ExecutionToProtoMessages(executions)...)
	}
	executionList, err := cmdCtx.AdminFetcherExt().ListExecution(ctx, config.GetConfig().Project, config.GetConfig().Domain, execution.DefaultConfig.Filter)
	if err != nil {
		return err
	}
	logger.Infof(ctx, "Retrieved %v executions", len(executionList.Executions))
	return adminPrinter.Print(config.GetConfig().MustOutputFormat(), executionColumns,
		ExecutionToProtoMessages(executionList.Executions)...)
}

func getNodeExecDetailsWithTasks(ctx context.Context, project, domain, name string, cmdCtx cmdCore.CommandContext)(
	[]*admin.NodeExecution, map[string]*admin.TaskExecutionList, error) {
	// Fetching Node execution details
	nExecDetails, err := cmdCtx.AdminFetcherExt().FetchNodeExecutionDetails(ctx, name, project, domain)
	if err != nil {
		return nil, nil, err
	}
	logger.Infof(ctx, "Retrieved %v node executions", len(nExecDetails.NodeExecutions))

	// Mapping node execution id to task list
	nodeExecToTaskExec := map[string]*admin.TaskExecutionList{}
	for _,nodeExec := range nExecDetails.NodeExecutions {
		nodeExecToTaskExec[nodeExec.Id.NodeId], err = cmdCtx.AdminFetcherExt().FetchTaskExecutionsOnNode(ctx,
			nodeExec.Id.NodeId, name, project, domain)
		if err != nil {
			return nil, nil, err
		}
	}
	return nExecDetails.NodeExecutions, nodeExecToTaskExec, nil
}

func printNodeDetailsTreeView(nodeExecutions []*admin.NodeExecution, nodeExecToTaskExec map[string]*admin.TaskExecutionList) {
	treeViewExec := gotree.New("Node Executions")
	for _,nodeExec := range nodeExecutions {
		nExecPhaseView := treeViewExec.Add(nodeExec.Id.NodeId+" - "+ nodeExec.Closure.Phase.String() +
			" - " + nodeExec.Closure.StartedAt.AsTime().String() +
			" - " + nodeExec.Closure.StartedAt.AsTime().
			Add(nodeExec.Closure.Duration.AsDuration()).String())
		taskExecs := nodeExecToTaskExec[nodeExec.Id.NodeId]
		if taskExecs != nil && len(taskExecs.TaskExecutions) > 0 {
			sort.Slice(taskExecs.TaskExecutions[:], func(i, j int) bool {
				return taskExecs.TaskExecutions[i].Id.RetryAttempt < taskExecs.TaskExecutions[j].Id.RetryAttempt
			})
			for _, taskExec := range taskExecs.TaskExecutions {
				attemptView := nExecPhaseView.Add("Attempt :" + strconv.Itoa(int(taskExec.Id.RetryAttempt)))
				attemptView.Add("Task - " + taskExec.Closure.Phase.String() +
					" - " + taskExec.Closure.StartedAt.AsTime().String() +
					" - " + taskExec.Closure.StartedAt.AsTime().
					Add(taskExec.Closure.Duration.AsDuration()).String())
				attemptView.Add("Task Type - " + taskExec.Closure.TaskType)
				attemptView.Add("Reason - " + taskExec.Closure.Reason)
				if  taskExec.Closure.Metadata != nil {
					metadata := attemptView.Add("Metadata")
					metadata.Add("Generated Name : " + taskExec.Closure.Metadata.GeneratedName)
					metadata.Add("Plugin Identifier : " + taskExec.Closure.Metadata.PluginIdentifier)
					extResourcesView := metadata.Add("External Resources")
					for _, extResource := range taskExec.Closure.Metadata.ExternalResources {
						extResourcesView.Add("Ext Resource : " + extResource.ExternalId)
					}
					resourcePoolInfoView := metadata.Add("Resource Pool Info")
					for _, rsPool := range taskExec.Closure.Metadata.ResourcePoolInfo {
						resourcePoolInfoView.Add("Ext Resource : " + rsPool.Namespace)
						resourcePoolInfoView.Add("Ext Resource : " + rsPool.AllocationToken)
					}
				}


				sort.Slice(taskExec.Closure.Logs[:], func(i, j int) bool {
					return taskExec.Closure.Logs[i].Name < taskExec.Closure.Logs[j].Name
				})

				logsView := attemptView.Add("Logs :")
				for _, logData := range taskExec.Closure.Logs {
					logsView.Add("Name :" + logData.Name)
					logsView.Add("URI :" + logData.Uri)
				}
			}
		}
	}
	fmt.Println(treeViewExec.Print())
}
