package comfyui

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Result is a finished ComfyUI image run (PNG bytes + metadata).
type Result struct {
	PNGBytes []byte
	Meta     map[string]any
}

// RunGenerateScene runs a scene workflow (source image optional if workflow supports text-only).
func (c *Client) RunGenerateScene(ctx context.Context, sourceURL string, input map[string]any) (*Result, error) {
	if c == nil {
		return nil, fmt.Errorf("comfyui: nil client")
	}
	return c.run(ctx, "generate_scene", sourceURL, input)
}

// RunReplaceBackground swaps background; requires a configured image node and reachable source image.
func (c *Client) RunReplaceBackground(ctx context.Context, sourceURL string, input map[string]any) (*Result, error) {
	if c == nil {
		return nil, fmt.Errorf("comfyui: nil client")
	}
	return c.run(ctx, "replace_background", sourceURL, input)
}

func (c *Client) run(ctx context.Context, mode, sourceURL string, input map[string]any) (*Result, error) {
	src := strings.TrimSpace(sourceURL)
	tpl := strings.TrimSpace(c.opts.WorkflowJSON)
	if !workflowConfigured(tpl) {
		return nil, fmt.Errorf("comfyui_workflow_json is empty or invalid")
	}

	if mode == "replace_background" {
		if strings.TrimSpace(c.opts.ImageNodeID) == "" || strings.TrimSpace(c.opts.OutputNodeID) == "" {
			return nil, fmt.Errorf("replace_background requires a configured ComfyUI workflow")
		}
		if src == "" {
			return nil, fmt.Errorf("replace_background requires sourceImageUrl")
		}
	}

	if strings.TrimSpace(c.opts.OutputNodeID) == "" {
		return nil, fmt.Errorf("comfyui_output_node_id is not configured")
	}

	vars := buildVarMap(input, src)
	expanded, err := expandWorkflowTemplate(tpl, vars)
	if err != nil {
		return nil, err
	}
	workflow, err := parseWorkflowObject(expanded)
	if err != nil {
		return nil, err
	}

	finalPrompt := strings.TrimSpace(stringFromVars(input, "assembled_prompt", "prompt"))
	if pid := strings.TrimSpace(c.opts.PromptNodeID); pid != "" {
		if err := setNodeTextPrompt(workflow, pid, finalPrompt); err != nil {
			return nil, err
		}
	}

	imgNode := strings.TrimSpace(c.opts.ImageNodeID)
	if src != "" && imgNode != "" {
		rawImg, err := c.downloadSource(ctx, src, 20<<20)
		if err != nil {
			return nil, err
		}
		upName, err := c.uploadImage(ctx, "trademind-source.png", rawImg)
		if err != nil {
			return nil, err
		}
		if err := setNodeLoadImage(workflow, imgNode, upName); err != nil {
			return nil, err
		}
	}

	promptID, err := c.postPrompt(ctx, workflow)
	if err != nil {
		return nil, err
	}

	pollCtx, cancel := context.WithTimeout(ctx, c.opts.MaxPoll)
	defer cancel()
	ticker := time.NewTicker(c.opts.PollInterval)
	defer ticker.Stop()

	var fn, sub, typ string
	tryFetch := func() (done bool, ferr error) {
		entry, ok, herr := c.getHistoryEntry(pollCtx, promptID)
		if herr != nil {
			return false, herr
		}
		if !ok || entry == nil {
			return false, nil
		}
		if msg := comfyFailureMessage(entry); msg != "" {
			return false, fmt.Errorf("comfyui execution failed: %s", msg)
		}
		f, s, t, err := firstOutputImage(entry, c.opts.OutputNodeID)
		if err != nil {
			return false, nil
		}
		fn, sub, typ = f, s, t
		return true, nil
	}

	if done, err := tryFetch(); err != nil {
		return nil, err
	} else if !done {
	pollLoop:
		for {
			select {
			case <-pollCtx.Done():
				return nil, fmt.Errorf("comfyui: timed out waiting for prompt %s", promptID)
			case <-ticker.C:
				done, err := tryFetch()
				if err != nil {
					return nil, err
				}
				if done {
					break pollLoop
				}
			}
		}
	}

	rawOut, _, err := c.downloadView(pollCtx, fn, sub, typ)
	if err != nil {
		return nil, err
	}

	pngBytes, err := encodeAsPNG(rawOut)
	if err != nil {
		// If already PNG and decode works through png decoder — jpeg path above handles.
		return nil, err
	}

	meta := map[string]any{
		"contentType": "image/png",
		"promptId":    promptID,
		"workflow":    "",
	}
	return &Result{
		PNGBytes: pngBytes,
		Meta:     meta,
	}, nil
}

func comfyFailureMessage(entry map[string]any) string {
	if entry == nil {
		return ""
	}
	st, ok := entry["status"].(map[string]any)
	if !ok || st == nil {
		return ""
	}
	if msg, ok := st["error"].(string); ok && strings.TrimSpace(msg) != "" {
		return strings.TrimSpace(msg)
	}
	if arr, ok := st["messages"].([]any); ok && len(arr) > 0 {
		if last, ok := arr[len(arr)-1].(map[string]any); ok {
			if m, ok := last["message"].(string); ok && strings.TrimSpace(m) != "" {
				return strings.TrimSpace(m)
			}
		}
	}
	return ""
}
