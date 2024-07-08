package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/muesli/reflow/wordwrap"
)

var (
	keywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Background(lipgloss.Color("235"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	green        = lipgloss.Color("#a6e3a1")
	red          = lipgloss.Color("#f38ba8")
	goodStyle    = lipgloss.NewStyle().Foreground(green)
	badStyle     = lipgloss.NewStyle().Foreground(red)
	mainStyle    = lipgloss.NewStyle()
)

type Status int

const (
	Neutral   Status = -1
	Good      Status = 1
	Bad       Status = 0
	API_URL          = "https://random-word-api.herokuapp.com/word?number=10"
	WORD_FILE        = "google-10000-english-usa-no-swears-medium.txt"
	ok_inputs        = " abcdefghijklmnopqrstuvwxyz"
)

type Model struct {
	chars    string
	words    []string
	inputs   []Status
	current  int
	quitting bool
	time     stopwatch.Model
	stats    string
}

func NewModel(words []string) Model {
	w, _, err := term.GetSize(0)
	if err != nil {
		panic("Couldn't get terminal size.")
	}
	s := wordwrap.String(strings.Join(words, " "), w)
	if len(s) == 0 {
		panic("Model creation error : zero words provided")
	}
	inputs := make([]Status, len(s))
	for index := range inputs {
		inputs[index] = Neutral
	}
	return Model{chars: s, words: words, inputs: inputs, quitting: false, time: stopwatch.NewWithInterval(time.Millisecond), stats: ""}

}

// GetWordsFromAPI grabs 10 random words from an online JSON API at API_URL.
// The words will most likely be long and used rarely.
func GetWordsFromAPI() []string {
	resp, err := http.Get(API_URL)
	if err != nil {
		panic("Error getting words")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic("Error reading word server response")
	}

	var words_list []string
	json.Unmarshal(body, &words_list)
	return words_list
}

// GetWordsFromFile grabs 10 random words from the file WORD_FILE.
// They are from the 10_000 most common American English.
// Source: https://github.com/first20hours/google-10000-english/blob/d0736d492489198e4f9d650c7ab4143bc14c1e9e/google-10000-english-usa-no-swears-medium.txt
func GetWordsFromFile() []string {
	// Open file
	content_b, err := os.ReadFile(WORD_FILE)
	if err != nil {
		panic("Couldn't read word file.")
	}
	lines := strings.Split(string(content_b), "\n")
	rand.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
	return lines[:10]
}

// initialModel gets 10 random lowercase english words and creates a model with them
func initialModel() Model {
	words := GetWordsFromFile()
	return NewModel(words)
}

// Render prints characters in red or green according to input
func (m Model) Render() string {
	result := ""
	for index, char := range m.chars {
		var style lipgloss.Style
		if index == m.current {
			style = mainStyle.Underline(true)
		}
		switch m.inputs[index] {
		case Bad:
			style = badStyle
		case Good:
			style = goodStyle
		}
		result += style.Render(string(char))
	}
	return result
}

// Quit stops the timer and compute some stats if appropriate
func (m *Model) Quit() {
	m.quitting = true
	if !m.time.Running() {
		return
	}
	m.time.Stop()
	//typed counts the number of words correctly typed
	typed := make([]Status, 0, len(m.words))
	flag := Good
	for index, curr_char := range m.chars {
		switch string(curr_char) {
		case " ":
			// We reached the end of a word
			typed = append(typed, flag)
			flag = Good
		default:
			if m.inputs[index] != Good {
				flag = Bad
			}
		}
	}
	typed = append(typed, flag)
	length_typed_chars := 0
	length_typed_words := 0
	for index, status := range typed {
		if status == Good {
			length_typed_words += 1
			length_typed_chars += len(m.words[index])
		}
	}

	var wpm float64 = float64(length_typed_chars) / (5 * m.time.Elapsed().Minutes())
	var accuracy float64
	for index := range m.current {
		accuracy += float64(m.inputs[index])
	}
	accuracy = accuracy / float64(m.current)
	m.stats = fmt.Sprintf("Correctly typed %v words in %.2fs.\nWPM: %.0f\nAccuracy: %.1f%%\nSee https://monkeytype.com/about for details about those stats.\n", length_typed_words, m.time.Elapsed().Seconds(), wpm, accuracy*100)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "esc":
			m.Quit()
			return m, tea.Quit
		default:
			if strings.Contains(ok_inputs, key) {
				if m.current == 0 && !m.time.Running() {
					cmd = m.time.Start()
				}
				var status Status = Neutral
				if string(m.chars[m.current]) == key {
					status = Good
				} else {
					status = Bad
				}
				m.inputs[m.current] = status
				if m.current == len(m.chars)-1 {
					m.Quit()
					return m, tea.Quit
				} else {
					m.current++
				}
			}
		}
	}
	if cmd == nil {
		m.time, cmd = m.time.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return m.Render() + "\n\n" + m.stats
	}
	return m.Render() + "\n\n" + helpStyle.MarginLeft(2).Render(fmt.Sprintf("%.2fs · esc, ^c: exit\n", m.time.Elapsed().Seconds()))
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
