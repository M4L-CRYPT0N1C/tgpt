package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/olekukonko/ts"
)

const localVersion = "1.5.1"

var bold = color.New(color.Bold)
var boldBlue = color.New(color.Bold, color.FgBlue)
var codeText = color.New(color.BgBlack, color.FgGreen)
var stopSpin = false

var programLoop = true
var serverID = ""
var configDir = ""
var userInput = ""

func main() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-terminate
		os.Exit(0)
	}()

	var err error
	hasConfig := true
	configDir, err = os.UserConfigDir()

	if err != nil {
		hasConfig = false
	}
	configTxtByte, err := os.ReadFile(configDir + "/tgpt/config.txt")
	if err != nil {
		hasConfig = false
	}
	chatId := ""
	if hasConfig {
		chatId = strings.Split(string(configTxtByte), ":")[1]
	}
	args := os.Args

	if len(args) > 1 && len(args[1]) > 1 {
		input := args[1]

		if input == "-v" || input == "--version" {
			fmt.Println("tgpt", localVersion)
		} else if input == "-s" || input == "--shell" {
			prompt := args[2]
			go loading(&stopSpin)
			formattedInput := strings.ReplaceAll(prompt, `"`, `\"`)
			formattedInput = strings.ReplaceAll(formattedInput, `\`, `\\`)
			shellCommand(formattedInput)
		} else if input == "-u" || input == "--update" {
			update()
		} else if input == "-i" || input == "--interactive" {
			/////////////////////
			// Normal interactive
			/////////////////////

			reader := bufio.NewReader(os.Stdin)
			bold.Println("Interactive mode started. Press Ctrl + C or type exit to quit.\n")
			serverID = chatId
			for {
				bold.Print(">> ")

				input, err := reader.ReadString('\n')
				if err != nil {
					fmt.Println("Error reading input:", err)
					break
				}

				if len(input) > 1 {
					input = strings.TrimSpace(input)
					if input == "exit" {
						bold.Println("Exiting...")
						return
					}
					serverID = getData(input, serverID, configDir+"/tgpt", true)

				}

			}

		} else if input == "-m" || input == "--multiline" {
			/////////////////////
			// Multiline interactive
			/////////////////////
			serverID = chatId

			fmt.Print("\nPress Tab to submit and Ctrl + C to exit.\n")

			for programLoop {
				fmt.Print("\n")
				p := tea.NewProgram(initialModel())
				_, err := p.Run()

				if err != nil {
					fmt.Println(err)
					os.Exit(0)
				}
				if len(userInput) > 0 {
					serverID = getData(userInput, serverID, configDir+"/tgpt", true)
				}

			}

		} else if input == "-f" || input == "--forget" {
			error := os.Remove(configDir + "/tgpt/config.txt")
			if error != nil {
				fmt.Println("There is no history to remove")
			} else {
				fmt.Println("Chat history removed")
			}
		} else if strings.HasPrefix(input, "-") {
			color.Blue(`Usage: tgpt [Flag] [Prompt]`)

			boldBlue.Println("\nFlags:")
			fmt.Printf("%-50v Generate and Execute shell commands. (Experimental) \n", "-s, --shell")

			boldBlue.Println("\nOptions:")
			fmt.Printf("%-50v Forget chat history \n", "-f, --forget")
			fmt.Printf("%-50v Print version \n", "-v, --version")
			fmt.Printf("%-50v Print help message \n", "-h, --help")
			fmt.Printf("%-50v Start normal interactive mode \n", "-i, --interactive")
			fmt.Printf("%-50v Start multi-line interactive mode \n", "-m, --multiline")
			if runtime.GOOS != "windows" {
				fmt.Printf("%-50v Update program \n", "-u, --update")
			}

			boldBlue.Println("\nExamples:")
			fmt.Println("tgpt -f")
			fmt.Println(`tgpt -s "How to update my system?"`)
		} else {
			go loading(&stopSpin)
			formattedInput := strings.ReplaceAll(input, `"`, `\"`)
			getData(formattedInput, chatId, configDir+"/tgpt", false)
		}

	} else {
		color.Red("You have to write some text")
		color.Blue(`Example: tgpt "Explain quantum computing in simple terms"`)
	}
}

// Multiline input

type errMsg error

type model struct {
	textarea textarea.Model
	err      error
}

func initialModel() model {
	size, _ := ts.GetSize()
	termWidth := size.Col()
	ti := textarea.New()
	ti.SetWidth(termWidth)
	ti.CharLimit = 200000
	ti.ShowLineNumbers = false
	ti.Placeholder = "Enter your prompt"
	ti.Focus()

	return model{
		textarea: ti,
		err:      nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.textarea.Focused() {
				m.textarea.Blur()
			}
		case tea.KeyCtrlC:
			programLoop = false
			userInput = ""
			return m, tea.Quit

		case tea.KeyTab:
			userInput = m.textarea.Value()

			if len(userInput) > 1 {
				m.textarea.Blur()
				return m, tea.Quit
			}

		default:
			if !m.textarea.Focused() {
				cmd = m.textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return m.textarea.View()
}

//////////////////////////////
