package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/lucas-clemente/quic-go/http3"
)

type Command struct {
	Operation  string            `json:"operation"`
	Parameters map[string]string `json:"parameters"`
	Timestamp  time.Time        `json:"timestamp"`
}

type Response struct {
	Status  string `json:"status"`
	Data    string `json:"data"`
	Message string `json:"message"`
}

type Terminal struct {
	theme           *material.Theme
	output          []string
	directoryInput  widget.Editor
	filterInput     widget.Editor
	tokenInput      widget.Editor
	clientIDInput   widget.Editor
	serverURLInput  widget.Editor
	executeButton   widget.Clickable
	outputList      widget.List
	outputEditor    widget.Editor
	client          *http.Client
}

func newTerminal() *Terminal {
	t := &Terminal{
		theme: material.NewTheme(gofont.Collection()),
		client: &http.Client{
			Transport: &http3.RoundTripper{},
			Timeout:   30 * time.Second,
		},
	}
	
	// Set default values
	t.serverURLInput.SetText("https://your-server-address/api/operations")
	t.directoryInput.SetText("/allowed/path")
	t.filterInput.SetText("*.txt")
	t.tokenInput.SetText("YOUR_AUTH_TOKEN")
	t.clientIDInput.SetText("YOUR_CLIENT_ID")
	
	t.outputEditor.SingleLine = false
	t.outputEditor.Submit = false
	t.outputList.Axis = layout.Vertical
	
	return t
}

func (t *Terminal) appendOutput(text string) {
	t.output = append(t.output, text)
	var builder strings.Builder
	for _, line := range t.output {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	t.outputEditor.SetText(builder.String())
}

func (t *Terminal) executeCommand() {
	cmd := Command{
		Operation: "list_files",
		Parameters: map[string]string{
			"directory": t.directoryInput.Text(),
			"filter":    t.filterInput.Text(),
		},
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(cmd)
	if err != nil {
		t.appendOutput(fmt.Sprintf("$ Error: Failed to marshal command: %v", err))
		return
	}

	req, err := http.NewRequest(
		"POST",
		t.serverURLInput.Text(),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		t.appendOutput(fmt.Sprintf("$ Error: Failed to create request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.tokenInput.Text())
	req.Header.Set("X-Client-ID", t.clientIDInput.Text())

	t.appendOutput(fmt.Sprintf("$ Executing command...\nURL: %s\nOperation: list_files\nDirectory: %s\nFilter: %s",
		t.serverURLInput.Text(), t.directoryInput.Text(), t.filterInput.Text()))

	resp, err := t.client.Do(req)
	if err != nil {
		t.appendOutput(fmt.Sprintf("$ Error: Failed to send request: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.appendOutput(fmt.Sprintf("$ Error: Failed to read response: %v", err))
		return
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.appendOutput(fmt.Sprintf("$ Error: Failed to parse response: %v", err))
		return
	}

	switch response.Status {
	case "success":
		t.appendOutput("$ Operation successful!")
		t.appendOutput(fmt.Sprintf("Result: %s", response.Data))
	case "error":
		t.appendOutput(fmt.Sprintf("$ Operation failed: %s", response.Message))
	default:
		t.appendOutput(fmt.Sprintf("$ Unexpected response status: %s", response.Status))
	}
}

func (t *Terminal) layout(gtx layout.Context) layout.Dimensions {
	// Define colors
	background := color.NRGBA{R: 40, G: 44, B: 52, A: 255}  // Dark background
	textColor := color.NRGBA{R: 171, G: 178, B: 191, A: 255} // Light text

	// Set theme colors
	t.theme.ContrastBg = background
	t.theme.Fg = textColor

	// Create border
	borderWidth := float32(1)
	borderColor := color.NRGBA{R: 80, G: 84, B: 92, A: 255}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.Fill(gtx.Ops, background)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Label(t.theme, unit.Sp(14), "Server URL:").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(t.theme, &t.serverURLInput, "")
								ed.Font.Style = text.Mono
								return ed.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),

							layout.Rigid(material.Label(t.theme, unit.Sp(14), "Directory:").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(t.theme, &t.directoryInput, "")
								ed.Font.Style = text.Mono
								return ed.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),

							layout.Rigid(material.Label(t.theme, unit.Sp(14), "Filter:").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(t.theme, &t.filterInput, "")
								ed.Font.Style = text.Mono
								return ed.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),

							layout.Rigid(material.Label(t.theme, unit.Sp(14), "Auth Token:").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(t.theme, &t.tokenInput, "")
								ed.Font.Style = text.Mono
								return ed.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),

							layout.Rigid(material.Label(t.theme, unit.Sp(14), "Client ID:").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(t.theme, &t.clientIDInput, "")
								ed.Font.Style = text.Mono
								return ed.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),

							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(t.theme, &t.executeButton, "Execute Command")
								return btn.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
						)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								paint.FillShape(gtx.Ops,
									borderColor,
									clip.Rect{
										Max: gtx.Constraints.Max,
									}.Op())
								return layout.Dimensions{Size: gtx.Constraints.Max}
							}),
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Stack{}.Layout(gtx,
										layout.Expanded(func(gtx layout.Context) layout.Dimensions {
											paint.FillShape(gtx.Ops,
												color.NRGBA{R: 30, G: 33, B: 40, A: 255},
												clip.Rect{
													Max: gtx.Constraints.Max,
												}.Op())
											return layout.Dimensions{Size: gtx.Constraints.Max}
										}),
										layout.Stacked(func(gtx layout.Context) layout.Dimensions {
											return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												ed := material.Editor(t.theme, &t.outputEditor, "")
												ed.Font.Style = text.Mono
												ed.TextSize = unit.Sp(14)
												return ed.Layout(gtx)
											})
										}),
									)
								})
							}),
						)
					}),
				)
			})
		}),
	)
}

func main() {
	go func() {
		w := app.NewWindow(
			app.Title("Terminal Emulator"),
			app.Size(unit.Dp(800), unit.Dp(600)),
		)

		term := newTerminal()
		var ops op.Ops

		for e := range w.Events() {
			switch e := e.(type) {
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)

				if term.executeButton.Clicked() {
					go term.executeCommand()
				}

				term.layout(gtx)
				e.Frame(gtx.Ops)

			case system.DestroyEvent:
				return
			}
		}
	}()
	app.Main()
}
