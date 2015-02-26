package main

type EventList []*JsonAppEvent

func (el EventList) Len() int { return len(el) }
func (el EventList) Less(i, j int) bool {
	return el[i].Timestamp.Before(el[j].Timestamp)
}
func (el EventList) Swap(i, j int) { el[i], el[j] = el[j], el[i] }
