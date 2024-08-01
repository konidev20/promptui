package promptui

import (
	"io"

	"github.com/chzyer/readline"
	"github.com/konidev20/promptui/list"
	"github.com/konidev20/promptui/screenbuf"
)

type MultiSelect struct {
	Select
	selected map[int]interface{}
}

func (ms *MultiSelect) Run() (map[int]interface{}, error) {
	if ms.Size == 0 {
		ms.Size = 5
	}

	l, err := list.New(ms.Items, ms.Size)
	if err != nil {
		return nil, err
	}
	l.Searcher = ms.Searcher

	ms.list = l
	ms.selected = make(map[int]interface{})

	ms.setKeys()

	err = ms.prepareTemplates()
	if err != nil {
		return nil, err
	}

	return ms.innerRun(ms.CursorPos, 0, ' ')
}

func (ms *MultiSelect) innerRun(cursorPos, scroll int, top rune) (map[int]interface{}, error) {
	c := &readline.Config{
		Stdin:  ms.Stdin,
		Stdout: ms.Stdout,
	}
	err := c.Init()
	if err != nil {
		return nil, err
	}

	c.Stdin = readline.NewCancelableStdin(c.Stdin)

	if ms.IsVimMode {
		c.VimMode = true
	}

	c.HistoryLimit = -1
	c.UniqueEditLine = true

	rl, err := readline.NewEx(c)
	if err != nil {
		return nil, err
	}

	rl.Write([]byte(hideCursor))
	sb := screenbuf.New(rl)

	cur := NewCursor("", ms.Pointer, false)

	canSearch := ms.Searcher != nil
	searchMode := ms.StartInSearchMode
	ms.list.SetCursor(cursorPos)
	ms.list.SetStart(scroll)

	c.SetListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		switch {
		case key == KeyEnter && !searchMode:
			return nil, 0, true
		case key == KeyEnter && searchMode:
			items, idx := ms.list.Items()
			if idx != list.NotFound {
				if _, exists := ms.selected[idx]; exists {
					delete(ms.selected, idx)
				} else {
					ms.selected[idx] = items[idx]
				}
			}
		case key == ' ' && !searchMode:
			items, idx := ms.list.Items()
			if idx != list.NotFound {
				if _, exists := ms.selected[idx]; exists {
					delete(ms.selected, idx)
				} else {
					ms.selected[idx] = items[idx]
				}
			}
		case key == ms.Keys.Next.Code || (key == 'j' && !searchMode):
			ms.list.Next()
		case key == ms.Keys.Prev.Code || (key == 'k' && !searchMode):
			ms.list.Prev()
		case key == ms.Keys.Search.Code:
			if !canSearch {
				break
			}

			if searchMode {
				searchMode = false
				cur.Replace("")
				ms.list.CancelSearch()
			} else {
				searchMode = true
			}
		case key == KeyBackspace || key == KeyCtrlH:
			if !canSearch || !searchMode {
				break
			}

			cur.Backspace()
			if len(cur.Get()) > 0 {
				ms.list.Search(cur.Get())
			} else {
				ms.list.CancelSearch()
			}
		case key == ms.Keys.PageUp.Code || (key == 'h' && !searchMode):
			ms.list.PageUp()
		case key == ms.Keys.PageDown.Code || (key == 'l' && !searchMode):
			ms.list.PageDown()
		default:
			if canSearch && searchMode {
				cur.Update(string(line))
				ms.list.Search(cur.Get())
			}
		}

		if searchMode {
			header := SearchPrompt + cur.Format()
			sb.WriteString(header)
		} else if !ms.HideHelp {
			help := ms.renderHelp(canSearch)
			sb.Write(help)
		}

		ms.renderItems(sb)

		return nil, 0, true
	})

	for {
		_, err = rl.Readline()

		if err != nil {
			switch {
			case err == readline.ErrInterrupt, err.Error() == "Interrupt":
				err = ErrInterrupt
			case err == io.EOF:
				err = ErrEOF
			}
			break
		}

		_, idx := ms.list.Items()
		if idx != list.NotFound {
			break
		}
	}

	if err != nil {
		if err.Error() == "Interrupt" {
			err = ErrInterrupt
		}
		sb.Reset()
		sb.WriteString("")
		sb.Flush()
		rl.Write([]byte(showCursor))
		rl.Close()
		return nil, err
	}

	items, idx := ms.list.Items()
	item := items[idx]

	if ms.HideSelected {
		clearScreen(sb)
	} else {
		sb.Reset()
		sb.Write(render(ms.Templates.selected, item))
		sb.Flush()
	}

	rl.Write([]byte(showCursor))
	rl.Close()

	return ms.selected, nil
}

func (ms *MultiSelect) renderItems(sb *screenbuf.ScreenBuf) {
	items, idx := ms.list.Items()
	last := len(items) - 1

	for i, item := range items {
		page := " "

		switch i {
		case 0:
			if ms.list.CanPageUp() {
				page = "↑"
			} else {
				page = " "
			}
		case last:
			if ms.list.CanPageDown() {
				page = "↓"
			}
		}

		output := []byte(page + " ")

		if _, selected := ms.selected[i]; selected {
			output = append(output, []byte("[x] ")...)
		} else {
			output = append(output, []byte("[ ] ")...)
		}

		if i == idx {
			output = append(output, render(ms.Templates.active, item)...)
		} else {
			output = append(output, render(ms.Templates.inactive, item)...)
		}

		sb.Write(output)
	}

	if idx == list.NotFound {
		sb.WriteString("")
		sb.WriteString("No results")
	} else {
		active := items[idx]

		details := ms.renderDetails(active)
		for _, d := range details {
			sb.Write(d)
		}
	}

	sb.Flush()
}
