package todo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	todoGroceryMemoryLimit      = 96
	todoGroceryMemoryAliasLimit = 16
	todoGroceryMemoryTitleRunes = 96
)

// todoGroceryMemoryItem is local Dash-Go catalog metadata. It is never sent to
// Microsoft Graph: only an actual Grocery task may sync to an external list.
type todoGroceryMemoryItem struct {
	Title    string   `json:"title"`
	Key      string   `json:"key"`
	Aliases  []string `json:"aliases,omitempty"`
	Uses     int      `json:"uses"`
	LastUsed int64    `json:"lastUsed"`
	Pinned   bool     `json:"pinned,omitempty"`
	Hidden   bool     `json:"hidden,omitempty"`
}

func todoGroceryMemoryTitle(v string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
	chars := []rune(value)
	if len(chars) > todoGroceryMemoryTitleRunes {
		return string(chars[:todoGroceryMemoryTitleRunes])
	}
	return value
}

func todoGroceryMemoryKey(v string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(v)), " "))
}

func todoGroceryMemoryEditableTitle(v string) (string, error) {
	value := strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
	if len([]rune(value)) > todoGroceryMemoryTitleRunes {
		return "", fmt.Errorf("quick add item name must be %d characters or fewer", todoGroceryMemoryTitleRunes)
	}
	return value, nil
}

func todoGroceryMemoryEditableKey(v string) (string, error) {
	value := todoGroceryMemoryKey(v)
	if len([]rune(value)) > todoGroceryMemoryTitleRunes {
		return "", fmt.Errorf("quick add item key must be %d characters or fewer", todoGroceryMemoryTitleRunes)
	}
	return value, nil
}

func todoGroceryMemoryAliases(values []string, key string) []string {
	seen := map[string]bool{key: true}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := todoGroceryMemoryKey(raw)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= todoGroceryMemoryAliasLimit {
			break
		}
	}
	slices.Sort(out)
	return out
}

func normalizeTodoGroceryMemoryItem(raw todoGroceryMemoryItem) (todoGroceryMemoryItem, bool) {
	raw.Title = todoGroceryMemoryTitle(raw.Title)
	raw.Key = todoGroceryMemoryKey(raw.Key)
	if raw.Key == "" {
		raw.Key = todoGroceryMemoryKey(raw.Title)
	}
	if raw.Title == "" || raw.Key == "" {
		return todoGroceryMemoryItem{}, false
	}
	if raw.Uses < 0 {
		raw.Uses = 0
	}
	if raw.LastUsed < 0 {
		raw.LastUsed = 0
	}
	raw.Aliases = todoGroceryMemoryAliases(raw.Aliases, raw.Key)
	return raw, true
}

func todoGroceryMemoryFind(items []todoGroceryMemoryItem, key string) int {
	key = todoGroceryMemoryKey(key)
	if key == "" {
		return -1
	}
	for i, item := range items {
		if item.Key == key {
			return i
		}
		for _, alias := range item.Aliases {
			if alias == key {
				return i
			}
		}
	}
	return -1
}

func todoGroceryMemoryMerged(base, other todoGroceryMemoryItem) todoGroceryMemoryItem {
	base.Uses += other.Uses
	base.LastUsed = max(base.LastUsed, other.LastUsed)
	base.Pinned = base.Pinned || other.Pinned
	// A merge remains visible if either source is visible. Editing a visible
	// suggestion into the label of a hidden one should honor the visible edit
	// rather than unexpectedly hiding the result.
	base.Hidden = base.Hidden && other.Hidden
	aliases := append([]string{}, base.Aliases...)
	aliases = append(aliases, base.Key, other.Key)
	aliases = append(aliases, other.Aliases...)
	base.Aliases = todoGroceryMemoryAliases(aliases, base.Key)
	return base
}

func todoGroceryMemoryDisplaySort(items []todoGroceryMemoryItem) {
	slices.SortStableFunc(items, func(left, right todoGroceryMemoryItem) int {
		if left.Hidden != right.Hidden {
			return compareBoolFalseFirst(left.Hidden, right.Hidden)
		}
		if left.Pinned != right.Pinned {
			return compareBoolTrueFirst(left.Pinned, right.Pinned)
		}
		if left.Uses != right.Uses {
			return compareIntsDescending(left.Uses, right.Uses)
		}
		if left.LastUsed != right.LastUsed {
			return compareInt64sDescending(left.LastUsed, right.LastUsed)
		}
		return compareFoldedText(left.Title, right.Title)
	})
}

func todoGroceryMemoryTrim(items []todoGroceryMemoryItem) []todoGroceryMemoryItem {
	if len(items) <= todoGroceryMemoryLimit {
		todoGroceryMemoryDisplaySort(items)
		return items
	}
	keep := append([]todoGroceryMemoryItem{}, items...)
	slices.SortStableFunc(keep, func(left, right todoGroceryMemoryItem) int {
		if left.Pinned != right.Pinned {
			return compareBoolTrueFirst(left.Pinned, right.Pinned)
		}
		if left.Hidden != right.Hidden {
			return compareBoolTrueFirst(left.Hidden, right.Hidden)
		}
		if left.Uses != right.Uses {
			return compareIntsDescending(left.Uses, right.Uses)
		}
		return compareInt64sDescending(left.LastUsed, right.LastUsed)
	})
	keep = keep[:todoGroceryMemoryLimit]
	todoGroceryMemoryDisplaySort(keep)
	return keep
}

func todoGroceryMemoryNormalize(items []todoGroceryMemoryItem) []todoGroceryMemoryItem {
	out := make([]todoGroceryMemoryItem, 0, len(items))
	for _, raw := range items {
		item, ok := normalizeTodoGroceryMemoryItem(raw)
		if !ok {
			continue
		}
		if i := todoGroceryMemoryFind(out, item.Key); i >= 0 {
			out[i] = todoGroceryMemoryMerged(out[i], item)
			continue
		}
		out = append(out, item)
	}
	return todoGroceryMemoryTrim(out)
}

func todoGroceryMemoryFromRaw(raw any) []todoGroceryMemoryItem {
	if raw == nil {
		return []todoGroceryMemoryItem{}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return []todoGroceryMemoryItem{}
	}
	var items []todoGroceryMemoryItem
	if err := json.Unmarshal(body, &items); err != nil {
		return []todoGroceryMemoryItem{}
	}
	return todoGroceryMemoryNormalize(items)
}

func todoGroceryMemoryStored(items []todoGroceryMemoryItem) []any {
	stored := make([]any, 0, len(items))
	for _, item := range todoGroceryMemoryNormalize(items) {
		stored = append(stored, item)
	}
	return stored
}

func (a *Service) todoGroceryMemory() []todoGroceryMemoryItem {
	return todoGroceryMemoryFromRaw(a.todoSettings()["groceryMemory"])
}

func (a *Service) mutateTodoGroceryMemory(mut func([]todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error)) ([]todoGroceryMemoryItem, error) {
	var next []todoGroceryMemoryItem
	_, err := a.mutateSettings(func(settings map[string]any) error {
		todo, _ := settings["todo"].(map[string]any)
		if todo == nil {
			todo = map[string]any{}
		}
		var mutationErr error
		next, mutationErr = mut(todoGroceryMemoryFromRaw(todo["groceryMemory"]))
		if mutationErr != nil {
			return mutationErr
		}
		next = todoGroceryMemoryNormalize(next)
		todo["groceryMemory"] = todoGroceryMemoryStored(next)
		settings["todo"] = todo
		return nil
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

func (a *Service) addTodoGroceryMemoryItem(title string) ([]todoGroceryMemoryItem, error) {
	var err error
	title, err = todoGroceryMemoryEditableTitle(title)
	if err != nil {
		return nil, err
	}
	key := todoGroceryMemoryKey(title)
	if key == "" {
		return nil, fmt.Errorf("quick add item name required")
	}
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		if i := todoGroceryMemoryFind(items, key); i >= 0 {
			if items[i].Hidden {
				items[i].Hidden = false
				items[i].LastUsed = todoNowMillis()
			}
			return items, nil
		}
		return append(items, todoGroceryMemoryItem{Title: title, Key: key, LastUsed: todoNowMillis()}), nil
	})
}

func (a *Service) editTodoGroceryMemoryItem(currentKey, title string) ([]todoGroceryMemoryItem, error) {
	var err error
	currentKey, err = todoGroceryMemoryEditableKey(currentKey)
	if err != nil {
		return nil, err
	}
	title, err = todoGroceryMemoryEditableTitle(title)
	if err != nil {
		return nil, err
	}
	nextKey := todoGroceryMemoryKey(title)
	if currentKey == "" || nextKey == "" {
		return nil, fmt.Errorf("quick add item name required")
	}
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		i := todoGroceryMemoryFind(items, currentKey)
		if i < 0 {
			return nil, fmt.Errorf("quick add item was not found")
		}
		item := items[i]
		item.Title = title
		aliases := append([]string{}, item.Aliases...)
		aliases = append(aliases, item.Key)
		item.Key = nextKey
		item.Aliases = todoGroceryMemoryAliases(aliases, item.Key)
		for j := len(items) - 1; j >= 0; j-- {
			if j == i {
				continue
			}
			if todoGroceryMemoryFind([]todoGroceryMemoryItem{items[j]}, nextKey) < 0 {
				continue
			}
			item = todoGroceryMemoryMerged(item, items[j])
			items = append(items[:j], items[j+1:]...)
			if j < i {
				i--
			}
		}
		items[i] = item
		return items, nil
	})
}

func (a *Service) setTodoGroceryMemoryPinned(key string, pinned bool) ([]todoGroceryMemoryItem, error) {
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		i := todoGroceryMemoryFind(items, key)
		if i < 0 {
			return nil, fmt.Errorf("quick add item was not found")
		}
		items[i].Pinned = pinned
		return items, nil
	})
}

func (a *Service) hideTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		i := todoGroceryMemoryFind(items, key)
		if i < 0 {
			return nil, fmt.Errorf("quick add item was not found")
		}
		items[i].Hidden = true
		items[i].Pinned = false
		return items, nil
	})
}

func (a *Service) restoreTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		i := todoGroceryMemoryFind(items, key)
		if i < 0 {
			return nil, fmt.Errorf("quick add item was not found")
		}
		items[i].Hidden = false
		items[i].LastUsed = todoNowMillis()
		return items, nil
	})
}

// deleteTodoGroceryMemoryItem permanently removes only a suggestion the user
// already hid. Visible suggestions retain the reversible Hide path, while a
// deletion removes its aliases and allows a future completed task to learn a
// fresh suggestion with the same title.
func (a *Service) deleteTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		i := todoGroceryMemoryFind(items, key)
		if i < 0 {
			return nil, fmt.Errorf("quick add item was not found")
		}
		if !items[i].Hidden {
			return nil, fmt.Errorf("hide the quick add item before deleting it")
		}
		return append(items[:i], items[i+1:]...), nil
	})
}

// todoRememberGroceryTasks learns only visible reusable suggestions. A hidden
// suggestion remains an explicit user choice and is never recreated by a later
// completed task with the same title or alias.
func (a *Service) todoRememberGroceryTasks(tasks []todoTask) {
	if len(tasks) == 0 {
		return
	}
	_, _ = a.mutateTodoGroceryMemory(func(items []todoGroceryMemoryItem) ([]todoGroceryMemoryItem, error) {
		now := todoNowMillis()
		for _, task := range tasks {
			title := todoGroceryMemoryTitle(task.Title)
			key := todoGroceryMemoryKey(title)
			if key == "" {
				continue
			}
			if i := todoGroceryMemoryFind(items, key); i >= 0 {
				if items[i].Hidden {
					continue
				}
				items[i].Uses++
				items[i].LastUsed = now
				continue
			}
			items = append(items, todoGroceryMemoryItem{Title: title, Key: key, Uses: 1, LastUsed: now})
		}
		return items, nil
	})
}
