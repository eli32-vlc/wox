package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiChatIcon = common.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats           []common.AIChatData
	chatsMu         sync.RWMutex
	agents          []common.AIAgent
	resultChatIdMap *util.HashMap[string /*chat id*/, string /*result id*/] // map of result id and chat id, used to update the chat title
	mcpServers      []common.AIChatMCPServerConfig
	mcpToolsMap     []common.MCPTool
	api             plugin.API

	chatCancelFuncs *util.HashMap[string, context.CancelFunc] // chat id -> cancel func, used to stop ongoing chats

	// Two-tier tool system: builtin + discovery hubs, with dynamic per-item tools
	builtinTools        []common.MCPTool           // always-present tools (macOS + system)
	discoveryTools      []common.MCPTool           // hub tools (may be retired on select)
	retiredDiscovery    map[string]bool            // names of discovery tools retired by selection
	dynamicTools        map[string][]common.MCPTool // selected item id -> generated tools
	dynamicToolOrder    []string                   // LRU order, most recent at end
	dynamicToolsMu      sync.RWMutex
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "i18n:plugin_ai_chat_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_ai_chat_plugin_description",
		Icon:            aiChatIcon.String(),
		TriggerKeywords: []string{"*", "chat"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "enable_fallback_search",
					DefaultValue: "true",
					Label:        "i18n:plugin_ai_chat_enable_fallback_search",
					Tooltip:      "i18n:plugin_ai_chat_enable_fallback_search_tooltip",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "ai_only_mode",
					DefaultValue: "false",
					Label:        "i18n:plugin_ai_chat_ai_only_mode",
					Tooltip:      "i18n:plugin_ai_chat_ai_only_mode_tooltip",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "enable_auto_focus_to_chat_input",
					DefaultValue: "true",
					Label:        "i18n:plugin_ai_chat_query_focus",
					Tooltip:      "i18n:plugin_ai_chat_query_focus_tooltip",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:     "default_model",
					Label:   "i18n:plugin_ai_chat_default_model",
					Tooltip: "i18n:plugin_ai_chat_default_model_tooltip",
					Style: definition.PluginSettingValueStyle{
						PaddingBottom: 8,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "agents",
					Title:   "i18n:plugin_ai_chat_agents",
					Tooltip: "i18n:plugin_ai_chat_agents_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "icon",
							Label:   "i18n:plugin_ai_chat_agent_icon",
							Type:    definition.PluginSettingValueTableColumnTypeWoxImage,
							Width:   45,
							Tooltip: "i18n:plugin_ai_chat_agent_icon_tooltip",
						},
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_chat_agent_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "prompt",
							Label:        "i18n:plugin_ai_chat_agent_prompt",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Tooltip:      "i18n:plugin_ai_chat_agent_prompt_tooltip",
						},
						{
							Key:     "model",
							Label:   "i18n:plugin_ai_chat_agent_model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_model_tooltip",
						},
						{
							Key:     "tools",
							Label:   "i18n:plugin_ai_chat_agent_tools",
							Type:    definition.PluginSettingValueTableColumnTypeAISelectMCPServerTools,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_tools_tooltip",
						},
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "mcp_servers",
					Title:   "i18n:plugin_ai_chat_mcp_servers",
					Tooltip: "i18n:plugin_ai_chat_mcp_servers_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_chat_mcp_server_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "tools",
							Label:        "i18n:plugin_ai_chat_mcp_server_tools",
							Tooltip:      "i18n:plugin_ai_chat_mcp_server_tools_tooltip",
							Type:         definition.PluginSettingValueTableColumnTypeAIMCPServerTools,
							Width:        50,
							HideInUpdate: true,
						},
						{
							Key:   "disabled",
							Label: "i18n:plugin_ai_chat_mcp_server_disabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 80,
						},
						{
							Key:     "type",
							Label:   "i18n:plugin_ai_chat_mcp_server_type",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   60,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_type_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{
									Label: "STDIO",
									Value: string(common.AIChatMCPServerTypeSTDIO),
								},
								{
									Label: "Streamable HTTP",
									Value: string(common.AIChatMCPServerTypeStreamableHTTP),
								},
							},
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "command",
							Label:   "i18n:plugin_ai_chat_mcp_server_command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_command_tooltip",
						},
						{
							Key:     "environmentVariables",
							Label:   "i18n:plugin_ai_chat_mcp_server_environment_variables",
							Type:    definition.PluginSettingValueTableColumnTypeTextList,
							Width:   160,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_environment_variables_tooltip",
						},
						{
							Key:          "url",
							Label:        "i18n:plugin_ai_chat_mcp_server_url",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Width:        80,
							Tooltip:      "i18n:plugin_ai_chat_mcp_server_url_tooltip",
						},
					},
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureAI,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]any{
					"WidthRatio": 0.25,
				},
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.resultChatIdMap = util.NewHashMap[string, string]()
	r.chatCancelFuncs = util.NewHashMap[string, context.CancelFunc]()
	r.api = initParams.API
	r.mcpServers = []common.AIChatMCPServerConfig{}

	chats, err := r.loadChats(ctx)
	if err != nil {
		r.chats = []common.AIChatData{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load chats: %s", err.Error()))
	} else {
		r.chats = chats
	}

	agents, err := r.loadAgents(ctx)
	if err != nil {
		r.agents = []common.AIAgent{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load agents: %s", err.Error()))
	} else {
		r.agents = agents
	}

	// Set AI-only mode from saved setting
	plugin.SetAIOnlyMode(r.api.GetSetting(ctx, "ai_only_mode") == "true")

	r.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == "ai_only_mode" {
			plugin.SetAIOnlyMode(value == "true")
		}

		if key == "agents" {
			agents, err := r.loadAgents(callbackCtx)
			if err != nil {
				r.api.Log(callbackCtx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load agents: %s", err.Error()))
				return
			}

			r.agents = agents

			plugin.GetPluginManager().GetUI().ReloadChatResources(callbackCtx, "agents")
		}

		if key == "mcp_servers" {
			r.reloadMCPServers(callbackCtx)
		}
	})

	// Delay MCP servers reload to avoid websocket server initialization race condition
	util.Go(ctx, "reload MCP servers", func() {
		time.Sleep(time.Millisecond * 1000) // Wait for websocket server to be ready
		r.reloadMCPServers(util.NewTraceContext())

		// Run discovery startup scan after tools are loaded
		time.Sleep(time.Millisecond * 2000) // Let MCP tools fully register
		RunDiscoveryStartupScan(util.NewTraceContext())
	})
}

func (r *AIChatPlugin) IsAutoFocusToChatInputWhenOpenWithQueryHotkey(ctx context.Context) bool {
	enableAutoFocusToChatInput := r.api.GetSetting(ctx, "enable_auto_focus_to_chat_input")
	return enableAutoFocusToChatInput == "true"
}

func (r *AIChatPlugin) RefreshDiscoveryCache(ctx context.Context) {
	discoveryCache.Clear()
	RunDiscoveryStartupScan(ctx)
}

func (r *AIChatPlugin) QueryFallback(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return nil
}

func (r *AIChatPlugin) GetDefaultModel(ctx context.Context) common.Model {
	model := r.api.GetSetting(context.Background(), "default_model")
	if model != "" {
		var m common.Model
		err := json.Unmarshal([]byte(model), &m)
		if err == nil {
			return m
		} else {
			r.api.Log(context.Background(), plugin.LogLevelError, fmt.Sprintf("AI: Failed to unmarshal default model: %s", err.Error()))
		}
	}

	// get last chat model
	if len(r.chats) > 0 {
		lastChat := r.chats[0]
		return common.Model{
			Name:          lastChat.Model.Name,
			Provider:      lastChat.Model.Provider,
			ProviderAlias: lastChat.Model.ProviderAlias,
		}
	}

	return common.Model{}
}

func (r *AIChatPlugin) reloadMCPServers(ctx context.Context) {
	r.api.Log(ctx, plugin.LogLevelInfo, "AI: Reloading MCP servers")

	mcpServers, err := r.loadMCPServers(ctx)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load mcp servers: %s", err.Error()))
	} else {
		r.mcpServers = mcpServers
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Loaded %d mcp servers", len(r.mcpServers)))
	}

	var mcpTools []common.MCPTool
	for _, mcpServer := range r.mcpServers {
		if mcpServer.Disabled {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: MCP server %s is disabled", mcpServer.Name))
			continue
		}

		tools, err := ai.MCPListTools(ctx, mcpServer)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to list tool: %s", err.Error()))
		}

		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Found %d tools for MCP server %s", len(tools), mcpServer.Name))
		for _, tool := range tools {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: %s tool %s", mcpServer.Name, tool.Name))
			mcpTools = append(mcpTools, tool)
		}
	}

	// Load always-present built-in tools (macOS + system)
	macosTools := GetMacOSTools()
	systemTools := GetSystemTools()
	r.builtinTools = append([]common.MCPTool{}, macosTools...)
	r.builtinTools = append(r.builtinTools, systemTools...)

	// Load hub discovery tools with dynamic tool registration
	r.discoveryTools = GetDiscoveryTools(r.dynamicToolRegistrar())

	r.dynamicToolsMu.Lock()
	r.retiredDiscovery = make(map[string]bool)
	r.dynamicTools = make(map[string][]common.MCPTool)
	r.dynamicToolOrder = nil
	r.dynamicToolsMu.Unlock()

	// mcpToolsMap kept for backward compat — all available tools
	mcpTools = append(mcpTools, r.builtinTools...)
	for _, dt := range r.discoveryTools {
		mcpTools = append(mcpTools, dt)
	}
	r.mcpToolsMap = mcpTools

	plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "tools")
}

func (r *AIChatPlugin) dynamicToolRegistrar() common.DynamicToolRegistrar {
	return common.DynamicToolRegistrar{
		Register:   r.registerDynamicTools,
		Unregister: r.unregisterDynamicTools,
		RetireDiscovery: func(ctx context.Context, toolName string) {
			r.dynamicToolsMu.Lock()
			r.retiredDiscovery[toolName] = true
			r.dynamicToolsMu.Unlock()
		},
	}
}

func (r *AIChatPlugin) registerDynamicTools(ctx context.Context, itemId string, tools []common.MCPTool) {
	r.dynamicToolsMu.Lock()
	defer r.dynamicToolsMu.Unlock()
	r.dynamicTools[itemId] = tools

	for i, id := range r.dynamicToolOrder {
		if id == itemId {
			r.dynamicToolOrder = append(r.dynamicToolOrder[:i], r.dynamicToolOrder[i+1:]...)
			break
		}
	}
	r.dynamicToolOrder = append(r.dynamicToolOrder, itemId)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Registered %d dynamic tools for item %s", len(tools), itemId))
}

func (r *AIChatPlugin) unregisterDynamicTools(ctx context.Context, itemId string) {
	r.dynamicToolsMu.Lock()
	defer r.dynamicToolsMu.Unlock()
	if itemId == "*" {
		r.dynamicTools = make(map[string][]common.MCPTool)
		r.dynamicToolOrder = nil
		r.retiredDiscovery = make(map[string]bool)
		r.api.Log(ctx, plugin.LogLevelInfo, "AI: Unregistered ALL dynamic tools and reset discovery")
		return
	}
	delete(r.dynamicTools, itemId)
	for i, id := range r.dynamicToolOrder {
		if id == itemId {
			r.dynamicToolOrder = append(r.dynamicToolOrder[:i], r.dynamicToolOrder[i+1:]...)
			break
		}
	}
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Unregistered dynamic tools for item %s", itemId))
}

func (r *AIChatPlugin) getActiveTools(ctx context.Context) []common.MCPTool {
	r.dynamicToolsMu.RLock()
	defer r.dynamicToolsMu.RUnlock()

	// Start with always-present builtin tools
	tools := make([]common.MCPTool, len(r.builtinTools))
	copy(tools, r.builtinTools)

	// Add non-retired discovery hub tools
	for _, dt := range r.discoveryTools {
		if !r.retiredDiscovery[dt.Name] {
			tools = append(tools, dt)
		}
	}

	// Calculate how many dynamic tools fit within provider cap
	providerCap := getProviderToolCap()
	remaining := providerCap - len(tools)
	if remaining <= 0 {
		return tools
	}

	// Add dynamic tools newest-first, up to capacity
	for i := len(r.dynamicToolOrder) - 1; i >= 0 && remaining > 0; i-- {
		itemId := r.dynamicToolOrder[i]
		dynTools, ok := r.dynamicTools[itemId]
		if !ok {
			continue
		}
		if len(dynTools) <= remaining {
			tools = append(tools, dynTools...)
			remaining -= len(dynTools)
		} else {
			tools = append(tools, dynTools[:remaining]...)
			remaining = 0
		}
	}

	return tools
}

func getProviderToolCap() int {
	return 200
}

func (r *AIChatPlugin) loadMCPServers(ctx context.Context) ([]common.AIChatMCPServerConfig, error) {
	mcpServersJson := r.api.GetSetting(ctx, "mcp_servers")
	if mcpServersJson == "" {
		return []common.AIChatMCPServerConfig{}, nil
	}

	var mcpServers []common.AIChatMCPServerConfig
	err := json.Unmarshal([]byte(mcpServersJson), &mcpServers)
	if err != nil {
		return []common.AIChatMCPServerConfig{}, err
	}

	return mcpServers, nil
}

func (r *AIChatPlugin) loadChats(ctx context.Context) ([]common.AIChatData, error) {
	chats := []common.AIChatData{}
	chatsJson := r.api.GetSetting(ctx, aiChatsSettingKey)
	if chatsJson == "" {
		return []common.AIChatData{}, nil
	}

	err := json.Unmarshal([]byte(chatsJson), &chats)
	if err != nil {
		return []common.AIChatData{}, err
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].UpdatedAt > chats[j].UpdatedAt
	})

	return chats, nil
}

func (r *AIChatPlugin) saveChats(ctx context.Context) {
	r.chatsMu.RLock()
	chatsJson, err := json.Marshal(r.chats)
	r.chatsMu.RUnlock()
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chats: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, aiChatsSettingKey, string(chatsJson), false)
}

func (r *AIChatPlugin) loadAgents(ctx context.Context) ([]common.AIAgent, error) {
	agents := []common.AIAgent{}
	agentsJson := r.api.GetSetting(ctx, "agents")
	if agentsJson == "" {
		return []common.AIAgent{}, nil
	}

	gjson.Parse(agentsJson).ForEach(func(_, agent gjson.Result) bool {
		gModel := gjson.Parse(agent.Get("model").String())
		modelName := gModel.Get("Name").String()
		modelProvider := gModel.Get("Provider").String()
		modelProviderAlias := gModel.Get("ProviderAlias").String()

		// Parse icon if available
		var icon common.WoxImage
		iconJson := agent.Get("icon").String()
		if iconJson != "" {
			gIcon := gjson.Parse(iconJson)
			icon = common.WoxImage{
				ImageType: gIcon.Get("ImageType").String(),
				ImageData: gIcon.Get("ImageData").String(),
			}
		} else {
			// Default icon if not set
			icon = common.WoxImage{
				ImageType: common.WoxImageTypeEmoji,
				ImageData: "🤖",
			}
		}

		agents = append(agents, common.AIAgent{
			Name:   agent.Get("name").String(),
			Prompt: agent.Get("prompt").String(),
			Model: common.Model{
				Name:          modelName,
				Provider:      common.ProviderName(modelProvider),
				ProviderAlias: modelProviderAlias,
			},
			Tools: lo.Map(agent.Get("tools").Array(), func(tool gjson.Result, _ int) string {
				return tool.String()
			}),
			Icon: icon,
		})
		return true
	})

	return agents, nil
}

func (r *AIChatPlugin) GetAllTools(ctx context.Context) []common.MCPTool {
	return r.mcpToolsMap
}

func (r *AIChatPlugin) GetAllAgents(ctx context.Context) []common.AIAgent {
	return r.agents
}

func (r *AIChatPlugin) Chat(ctx context.Context, aiChatData common.AIChatData, chatLoopCount int) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Starting chat with ID: %s, loop: %d, title: %s, model: %s, conversations: %d", aiChatData.Id, chatLoopCount, aiChatData.Title, aiChatData.Model.Name, len(aiChatData.Conversations)))

	if len(aiChatData.Tools) > 0 {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Selected tools: %v", aiChatData.Tools))
	}

	// Fall back to default model if none specified
	if aiChatData.Model.Name == "" {
		aiChatData.Model = r.GetDefaultModel(ctx)
	}

	// Add default system prompt for new conversations
	if chatLoopCount == 0 {
		hasAgentPrompt := false
		if aiChatData.AgentName != "" {
			for _, agent := range r.agents {
				if agent.Name == aiChatData.AgentName {
					r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Using agent: %s", agent.Name))
					hasAgentPrompt = true

					if agent.Prompt != "" {
						systemPrompt := common.Conversation{
							Id:        uuid.NewString(),
							Role:      common.ConversationRoleSystem,
							Text:      agent.Prompt,
							Timestamp: util.GetSystemTimestamp(),
						}
						aiChatData.Conversations = append([]common.Conversation{systemPrompt}, aiChatData.Conversations...)
					}

					if agent.Model.Name != "" {
						aiChatData.Model = agent.Model
					}

					if len(agent.Tools) > 0 {
						aiChatData.Tools = agent.Tools
					}
					break
				}
			}
		}

		// Add default system prompt if no agent prompt was set
		if !hasAgentPrompt {
			defaultPrompt := common.Conversation{
				Id:        uuid.NewString(),
				Role:      common.ConversationRoleSystem,
				Text:      `You are a macOS AI assistant with a two-tier tool system.

TIER 1 — ALWAYS AVAILABLE (~100 tools): System monitoring (disk, memory, CPU, battery, network, display, audio, Bluetooth, security), file operations, clipboard, shell, media, and general utilities.

TIER 2 — DYNAMIC HUB TOOLS (9 discovery tools): These let you find and interact with specific system resources. Each one supports an optional "select" parameter that generates per-item control tools:

- macos_search_apps — Find installed apps. Pass select=bundleId to get launch, URL open, and info tools for that app.
- macos_search_url_schemes — Find apps that handle URL schemes. Pass select=scheme to get an open tool.
- macos_search_services — Find launchd daemons. Pass select=name to get start/stop/status tools.
- macos_search_agents — Find launch agents. Pass select=name to get start/stop/status tools.
- macos_search_preferences — Find preference domains. Pass select=domain to get read/write tools.
- macos_search_homebrew — Find Homebrew packages. Pass select=name to get upgrade tools.
- macos_search_daemons — Find system daemons. Pass select=name to get start/stop/status tools.
- macos_search_file_types — Find apps for a file extension. Pass select=bundleId to get tools.
- macos_search_extensions — Find app extensions. (Coming soon.)

WORKFLOW:
1. Use a hub tool to search (e.g., macos_search_apps)
2. When you find the item, call the same hub tool with its "select" parameter
3. Control tools for that item become available in subsequent turns
4. Each selection replaces the hub tool with 2-5 control tools

CRITICAL RULES:
- ALWAYS use tools to gather real data — never guess or fabricate system information. Every system query must start with a tool call.
- When the user asks about their system status, files, apps, or settings, call the relevant tool first, then respond with the actual results.
- Be concise and accurate. Report raw tool output in a readable format.
- If a tool call fails, explain why and try an alternative approach or tool.
- If you need to control an app or service you haven't selected yet, use the relevant hub tool with "select".`,
				Timestamp: util.GetSystemTimestamp(),
			}
			aiChatData.Conversations = append([]common.Conversation{defaultPrompt}, aiChatData.Conversations...)
		}
	}

	r.appendOrUpdateChatData(aiChatData)
	r.saveChats(ctx)

	var tools []common.MCPTool
	if len(aiChatData.Tools) > 0 {
		tools = lo.Filter(r.mcpToolsMap, func(tool common.MCPTool, _ int) bool {
			return lo.Contains(aiChatData.Tools, tool.Name)
		})
	} else {
		tools = r.getActiveTools(ctx)
	}

	// Store cancel func so the chat can be stopped via /ai/chat/stop
	chatCtx, chatCancel := context.WithCancel(ctx)
	r.chatCancelFuncs.Store(aiChatData.Id, chatCancel)

	var responseId = uuid.NewString()
	chatErr := r.api.AIChatStream(chatCtx, aiChatData.Model, aiChatData.Conversations, common.ChatOptions{
		Tools: tools,
	}, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream receiving data, status: %s, data: %s", streamResult.Status, streamResult.Data))

		// update conversations and sync to UI
		if streamResult.Data != "" || streamResult.Reasoning != "" {
			r.appendOrUpdateConversation(&aiChatData, common.Conversation{
				Id:        responseId,
				Role:      common.ConversationRoleAssistant,
				Text:      streamResult.Data,
				Reasoning: streamResult.Reasoning,
				Timestamp: util.GetSystemTimestamp(),
			})
		}
		if len(streamResult.ToolCalls) > 0 {
			for _, toolCall := range streamResult.ToolCalls {
				r.appendOrUpdateConversation(&aiChatData, common.Conversation{
					Id:           toolCall.Id,
					Role:         common.ConversationRoleTool,
					Text:         toolCall.Delta,
					ToolCallInfo: toolCall,
					Timestamp:    toolCall.StartTimestamp,
				})
			}
		}
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)

		if streamResult.Status == common.ChatStreamStatusFinished {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream finished: %s", streamResult.Data))
			r.appendOrUpdateChatData(aiChatData)
			r.saveChats(ctx)

			// Clean up the cancel func when chat finishes
			r.chatCancelFuncs.Delete(aiChatData.Id)

			// only summarize the chat title if there is no tool call
			// if there is any toolcall, we need to wait for the tool call to finish
			if len(streamResult.ToolCalls) == 0 {
				r.summaryTitleIfNecessary(ctx, aiChatData)
			}

			if streamResult.IsAllToolCallsSucceeded() {
				// recursively call the chat to continue
				r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: recursively calling the chat to continue, loop: %d", chatLoopCount+1))
				r.Chat(ctx, aiChatData, chatLoopCount+1)
			}
		}
	})

	// Clean up cancel func when stream finishes (success or error)
	r.chatCancelFuncs.Delete(aiChatData.Id)

	if chatErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to chat: %s", chatErr.Error()))
		r.appendOrUpdateConversation(&aiChatData, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleAssistant,
			Text:      fmt.Sprintf(r.api.GetTranslation(ctx, "ui_ai_chat_error"), chatErr.Error()),
			Timestamp: util.GetSystemTimestamp(),
		})
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)
		r.appendOrUpdateChatData(aiChatData)
		r.saveChats(ctx)
		r.api.Notify(ctx, r.api.GetTranslation(ctx, "ui_ai_chat_failed_to_chat"))
	}
}

func (r *AIChatPlugin) summaryTitleIfNecessary(ctx context.Context, aiChatData common.AIChatData) {
	summarizeIndex := []int{2, 3, 4, 10}
	for _, index := range summarizeIndex {
		nonToolConversationCount := lo.CountBy(aiChatData.Conversations, func(conversation common.Conversation) bool {
			return conversation.Role != common.ConversationRoleTool
		})
		if nonToolConversationCount == index {
			r.summarizeChat(ctx, aiChatData)
			break
		}
	}

}

func (r *AIChatPlugin) appendOrUpdateConversation(aiChatData *common.AIChatData, conversation common.Conversation) {
	for i := range aiChatData.Conversations {
		if aiChatData.Conversations[i].Id == conversation.Id {
			aiChatData.Conversations[i] = conversation
			return
		}
	}

	aiChatData.Conversations = append(aiChatData.Conversations, conversation)
}

func (r *AIChatPlugin) appendOrUpdateChatData(aiChatData common.AIChatData) {
	r.chatsMu.Lock()
	defer r.chatsMu.Unlock()
	for i := range r.chats {
		if r.chats[i].Id == aiChatData.Id {
			r.chats[i] = aiChatData
			return
		}
	}

	r.chats = append(r.chats, aiChatData)
	sort.Slice(r.chats, func(i, j int) bool {
		return r.chats[i].UpdatedAt > r.chats[j].UpdatedAt
	})
}

func (r *AIChatPlugin) getNewChatPreviewData(ctx context.Context) plugin.QueryResult {
	var chatData common.AIChatData
	chatData.Id = uuid.NewString()
	chatData.Title = ""
	chatData.CreatedAt = util.GetSystemTimestamp()
	chatData.UpdatedAt = util.GetSystemTimestamp()
	chatData.Conversations = []common.Conversation{}
	chatData.Model = r.GetDefaultModel(ctx)

	previewData, err := json.Marshal(chatData)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chat preview data: %s", err.Error()))
		return plugin.QueryResult{}
	}

	resultId := uuid.NewString()
	r.resultChatIdMap.Store(chatData.Id, resultId)

	return plugin.QueryResult{
		Id:       resultId,
		Title:    "i18n:ui_ai_chat_new_chat",
		SubTitle: "i18n:ui_ai_chat_create_new_chat",
		Icon:     aiChatIcon,
		Preview: plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeChat,
			PreviewData:    string(previewData),
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
		},
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:ui_ai_chat_start_chat",
				PreventHideAfterAction: true,
				ContextData:            common.ContextData{"chatId": chatData.Id},
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
				},
			},
		},
		Group:      "i18n:ui_ai_chat_new_chat",
		GroupScore: 1000,
	}
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	// In AI-only mode, route all typed input as new AI chat messages
	if plugin.IsAIOnlyMode() && query.Search != "" {
		return r.newChatResultFromQuery(ctx, query.Search)
	}

	// Return saved chat sessions as search results when query is empty
	var results []plugin.QueryResult

	// Add "New Chat" entry always
	results = append(results, r.getNewChatPreviewData(ctx))

	// Add existing chat sessions
	for _, chat := range r.chats {
		chatData := chat
		if chatData.Title == "" {
			chatData.Title = "i18n:ui_ai_chat_new_chat"
		}

		previewData, err := json.Marshal(chatData)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chat preview data: %s", err.Error()))
			continue
		}

		resultId := uuid.NewString()
		r.resultChatIdMap.Store(chatData.Id, resultId)

		groupName, groupScore := r.getResultGroup(ctx, chatData)

		results = append(results, plugin.QueryResult{
			Id:       resultId,
			Title:    chatData.Title,
			SubTitle: fmt.Sprintf("%d conversations", len(chatData.Conversations)),
			Icon:     aiChatIcon,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeChat,
				PreviewData:    string(previewData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:ui_ai_chat_start_chat",
					PreventHideAfterAction: true,
					ContextData:            common.ContextData{"chatId": chatData.Id},
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
					},
				},
			},
			Group:      groupName,
			GroupScore: groupScore,
		})
	}

	return plugin.NewQueryResponse(results)
}

func (r *AIChatPlugin) newChatResultFromQuery(ctx context.Context, searchText string) plugin.QueryResponse {
	chatData := common.AIChatData{
		Id:        uuid.NewString(),
		Title:     searchText,
		CreatedAt: util.GetSystemTimestamp(),
		UpdatedAt: util.GetSystemTimestamp(),
		Model:     r.GetDefaultModel(ctx),
		Conversations: []common.Conversation{
			{
				Id:        uuid.NewString(),
				Role:      common.ConversationRoleUser,
				Text:      searchText,
				Timestamp: util.GetSystemTimestamp(),
			},
		},
	}

	r.appendOrUpdateChatData(chatData)
	r.saveChats(ctx)

	util.Go(ctx, "ai chat from global query", func() {
		r.Chat(util.NewTraceContext(), chatData, 0)
	})

	resultId := uuid.NewString()
	r.resultChatIdMap.Store(chatData.Id, resultId)
	previewData, _ := json.Marshal(chatData)

	return plugin.NewQueryResponse([]plugin.QueryResult{
		{
			Id:       resultId,
			Title:    searchText,
			SubTitle: "AI Chat",
			Icon:     aiChatIcon,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeChat,
				PreviewData:    string(previewData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Group:      "i18n:ui_ai_chat_new_chat",
			GroupScore: 1000,
		},
	})
}

func (r *AIChatPlugin) summarizeChat(ctx context.Context, chat common.AIChatData) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Summarizing chat: %s", chat.Id))

	var conversations []common.Conversation
	conversations = lo.Filter(chat.Conversations, func(conversation common.Conversation, _ int) bool {
		return conversation.Role != common.ConversationRoleTool
	})
	conversations = append(conversations, common.Conversation{
		Id:   uuid.NewString(),
		Role: common.ConversationRoleUser,
		Text: `Please summarize our conversation above and provide a clear and concise title. Requirements:
1. The title should be no more than 50 characters.
2. The language of the title should be the same as the language of the conversation.
3. The title should be a single sentence.
4. The response should be only the title, no other text.
`,
		Images:    []common.WoxImage{},
		Timestamp: util.GetSystemTimestamp(),
	})

	summarizeErr := r.api.AIChatStream(ctx, chat.Model, conversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat summarize stream data: %s", streamResult.Data))

		if streamResult.Status == common.ChatStreamStatusError {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: chat summarize stream error: %s", streamResult.Data))
			return
		}

		if streamResult.Status == common.ChatStreamStatusFinished {
			title := strings.TrimSpace(streamResult.Data)
			title = strings.ReplaceAll(title, "\n", "")
			title = strings.TrimPrefix(title, "\"")
			title = strings.TrimSuffix(title, "\"")

			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Summarized chat title: %s", title))

			if title == "" {
				return
			}

			r.chatsMu.Lock()
			for i := range r.chats {
				if r.chats[i].Id == chat.Id {
					r.chats[i].Title = title
					break
				}
			}
			r.chatsMu.Unlock()
			r.saveChats(ctx)

			if resultId, ok := r.resultChatIdMap.Load(chat.Id); ok {
				plugin.GetPluginManager().GetUI().UpdateResult(ctx, plugin.UpdatableResult{
					Id:    resultId,
					Title: &title,
				})
			}
		}
	})

	if summarizeErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to summarize chat: %s", summarizeErr.Error()))
	}
}

func (r *AIChatPlugin) StopChat(ctx context.Context, chatId string) bool {
	if cancelFunc, ok := r.chatCancelFuncs.Load(chatId); ok {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Stopping chat: %s", chatId))
		cancelFunc()
		r.chatCancelFuncs.Delete(chatId)
		return true
	}
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: No active chat found to stop: %s", chatId))
	return false
}

func (c *AIChatPlugin) getResultGroup(ctx context.Context, chat common.AIChatData) (string, int64) {
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24 {
		return "i18n:ui_ai_chat_history_today", 90
	}
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24*2 {
		return "i18n:ui_ai_chat_history_yesterday", 80
	}

	return "i18n:ui_ai_chat_history_history", 10
}
