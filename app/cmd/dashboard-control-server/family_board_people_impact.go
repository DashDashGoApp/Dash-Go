package main

// familyBoardPersonMessageCount intentionally returns only a count for the
// People impact/delete flow. It never exposes private message content.
func (a *app) familyBoardPersonMessageCount(personID string) int {
	return a.familyBoardService().PersonMessageCount(personID)
}
