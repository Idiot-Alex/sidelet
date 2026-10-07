//go:build windows || darwin

package main

import (
	"encoding/json"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestSharedPopupRoutesOnlyCurrentPresentation(t *testing.T) {
	w := new(application.WebviewWindow)
	c := &controller{sharedPopup: true, quick: &overlay{window: w}, add: &overlay{window: w}}
	add := c.popup.Prepare("add")
	if c.find(w) != c.add || !c.acceptPopupPacket(message{Window: w, Type: "quick-add-save", PopupView: "add", PopupRevision: add}) {
		t.Fatal("active add is not routed to its own logical view")
	}
	c.popup.Close("add")
	card := c.popup.Prepare("quick")
	if c.acceptPopupPacket(message{Window: w, Type: "popup-close", PopupView: "add", PopupRevision: add}) {
		t.Fatal("late native close can dismiss the replacement card")
	}
	if c.find(w) != c.quick || c.acceptPopupPacket(message{Window: w, Type: "quick-add-save", PopupView: "add", PopupRevision: add}) {
		t.Fatal("late add save can operate on the card")
	}
	if c.acceptPopupPacket(message{Window: w, Type: "regions", PopupView: "add", PopupRevision: card}) {
		t.Fatal("wrong view can resize the current input surface")
	}
	if !c.acceptPopupPacket(message{Window: w, Type: "action", PopupView: "quick", PopupRevision: card}) {
		t.Fatal("current card action was rejected")
	}
	c.popup.Close("quick")
	if c.acceptPopupPacket(message{Window: w, Type: "close-quick", PopupView: "quick", PopupRevision: card}) {
		t.Fatal("hidden popup accepted a delayed close")
	}
	if !c.acceptPopupPacket(message{Window: w, Type: "memory-view"}) || !c.acceptPopupPacket(message{Window: new(application.WebviewWindow), Type: "action"}) {
		t.Fatal("shared popup guard interfered with diagnostics or another window")
	}
}

func TestQuickCardIgnoresStaleResizeAndReplacementView(t *testing.T) {
	w := new(application.WebviewWindow)
	c := &controller{sharedPopup: true, quick: &overlay{window: w}, quickPlacement: &quickRequest{}, quickHeight: 180}
	c.quickSession.Begin("stack-0", "popup", 1)
	c.popup.Prepare("quick")
	current := message{Window: w, Revision: c.quickSession.RequestRevision, CardHeight: 220}
	old := current
	old.Revision--
	if err := c.resizeQuickCard(old); err != nil {
		t.Fatal(err)
	}
	c.quickPending = &quickRequest{}
	if err := c.resizeQuickCard(current); err != nil {
		t.Fatal(err)
	}
	c.quickPending = nil
	c.popup.Prepare("add")
	if err := c.resizeQuickCard(current); err != nil {
		t.Fatal(err)
	}
	c.popup.Prepare("quick")
	c.quickSession.Close()
	if err := c.resizeQuickCard(current); err != nil {
		t.Fatal(err)
	}
	if c.quickHeight != 180 {
		t.Fatal("late renderer resize changed a replacement or hidden card")
	}
	c.quickSession.Begin("stack-0", "popup", 1)
	c.quick.mode = "Editing"
	c.quickHeight = 390
	current.Revision = c.quickSession.RequestRevision
	current.CardHeight = 160
	if err := c.resizeQuickCard(current); err != nil {
		t.Fatal(err)
	}
	if c.quickHeight != 390 {
		t.Fatal("late reading measurement shrank the active editor")
	}
}

func TestQuickEditFinishRejectsClosedReplacedAndUnrelatedCards(t *testing.T) {
	w := new(application.WebviewWindow)
	c := &controller{sharedPopup: true, quick: &overlay{window: w, mode: "Editing"}}
	c.quickSession.Begin("stack-0", "popup", 1)
	card := c.popup.Prepare("quick")
	m := message{Type: "quick-edit-finish", Window: w, Revision: c.quickSession.RequestRevision, PopupView: "quick", PopupRevision: card}
	old := m
	old.Revision--
	if err := c.finishQuickEditing(old); err != nil {
		t.Fatal(err)
	}
	c.quickPending = &quickRequest{}
	if err := c.finishQuickEditing(m); err != nil {
		t.Fatal(err)
	}
	c.quickPending = nil
	c.popup.Prepare("add")
	if c.acceptPopupPacket(m) {
		t.Fatal("late edit finish can affect the replacement add window")
	}
	if err := c.finishQuickEditing(m); err != nil {
		t.Fatal(err)
	}
	c.popup.Prepare("quick")
	c.quickSession.Close()
	if err := c.finishQuickEditing(m); err != nil {
		t.Fatal(err)
	}
	if c.quick.mode != "Editing" {
		t.Fatal("stale edit finish changed interaction state")
	}
}

func TestSharedPopupAcceptsHostPointerWithoutRendererGeneration(t *testing.T) {
	w := new(application.WebviewWindow)
	c := &controller{sharedPopup: true, quick: &overlay{window: w}, add: &overlay{window: w}}
	card := c.popup.Prepare("quick")
	if !c.acceptPopupPacket(message{Window: w, Type: "pointer", Inside: true, nativePointer: true}) {
		t.Fatal("host card entry was dropped, so the leave timer cannot be cancelled")
	}
	if !c.acceptPopupPacket(message{Window: w, Type: "presence", Inside: true, PopupView: "quick", PopupRevision: card}) {
		t.Fatal("current renderer presence was rejected")
	}
	var forged message
	if err := json.Unmarshal([]byte(`{"type":"pointer","inside":true,"nativePointer":true}`), &forged); err != nil {
		t.Fatal(err)
	}
	forged.Window = w
	if c.acceptPopupPacket(forged) {
		t.Fatal("renderer JSON bypassed the generation guard")
	}
	c.popup.Close("quick")
	c.popup.Prepare("add")
	if c.acceptPopupPacket(message{Window: w, Type: "presence", Inside: true, PopupView: "quick", PopupRevision: card}) {
		t.Fatal("old card presence affected the replacement add view")
	}
}
