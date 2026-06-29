package main

import (
	"net/http"
	"os"
	"os/exec"
)

// runXset is an HTTP response adapter. Display scheduling/state remains in
// core because it is coupled to request authorization and response formatting.
func (a *app) runXset(w http.ResponseWriter, state string) {
	env := os.Environ()
	env = append(env, "DISPLAY=:0", "XAUTHORITY="+a.platformXAuthority())
	cmd := exec.Command("xset", "dpms", "force", state)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		a.err(w, "xset failed: "+string(out), http.StatusInternalServerError)
		return
	}
	a.json(w, map[string]any{"display": state})
}
