//go:build windows || darwin

// Package platform contains the Spike's native desktop boundary.
// Window operations run on the application's UI thread.
package platform

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Display struct {
	ID           string  `json:"id"`
	WorkArea     Rect    `json:"workArea"`
	Bounds       Rect    `json:"bounds"`
	Scale        float64 `json:"scale"` // native coordinate units per CSS pixel
	BackingScale float64 `json:"backingScale"`
	Primary      bool    `json:"primary"`
}

type Callbacks struct {
	Hotkey          func()
	TestKeyboard    func() // Explicit fixture entry; never a global-key assertion.
	Changed         func()
	Blur            func()
	Pointer         func(x, y float64, inside bool)
	CheckFullscreen func()
}
