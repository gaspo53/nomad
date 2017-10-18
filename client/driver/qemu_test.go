package driver

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func generateString(length int) string {
	var newString string
	for i := 0; i < length; i++ {
		newString = newString + "x"
	}
	return string(newString)
}

// The fingerprinter test should always pass, even if QEMU is not installed.
func TestQemuDriver_Fingerprint(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Driver:    "qemu",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes[qemuDriverAttr] == "" {
		t.Fatalf("Missing Qemu driver")
	}
	if node.Attributes[qemuDriverVersionAttr] == "" {
		t.Fatalf("Missing Qemu driver version")
	}
	if node.Attributes[qemuDriverLongMonitorPathAttr] == "" {
		t.Fatalf("Missing Qemu long monitor socket path support flag")
	}
}

func TestQemuDriver_StartOpen_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":        "linux-0.2.img",
			"accelerator":       "tcg",
			"graceful_shutdown": true,
			"port_map": []map[string]int{{
				"main": 22,
				"web":  8080,
			}},
			"args": []string{"-nodefconfig", "-nodefaults"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	// Copy the test image into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir

	copyFile("./test-resources/qemu/linux-0.2.img", filepath.Join(dst, "linux-0.2.img"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("Prestart failed: %v", err)
	}

	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that sending a Signal returns an error
	if err := resp.Handle.Signal(syscall.SIGINT); err == nil {
		t.Fatalf("Expect an error when signalling")
	}

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Clean up
	if err := resp.Handle.Kill(); err != nil {
		fmt.Printf("\nError killing Qemu test: %s", err)
	}
}

func TestQemuDriverUser(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	tasks := []*structs.Task{
		{
			Name:   "linux",
			Driver: "qemu",
			User:   "alice",
			Config: map[string]interface{}{
				"image_path":        "linux-0.2.img",
				"accelerator":       "tcg",
				"graceful_shutdown": false,
				"port_map": []map[string]int{{
					"main": 22,
					"web":  8080,
				}},
				"args": []string{"-nodefconfig", "-nodefaults"},
				"msg":  "unknown user alice",
			},
			LogConfig: &structs.LogConfig{
				MaxFiles:      10,
				MaxFileSizeMB: 10,
			},
			Resources: &structs.Resources{
				CPU:      500,
				MemoryMB: 512,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
		{
			Name:   "linux",
			Driver: "qemu",
			User:   "alice",
			Config: map[string]interface{}{
				"image_path":  "linux-0.2.img",
				"accelerator": "tcg",
				"port_map": []map[string]int{{
					"main": 22,
					"web":  8080,
				}},
				"args": []string{"-nodefconfig", "-nodefaults"},
				"msg":  "Qemu memory assignment out of bounds",
			},
			LogConfig: &structs.LogConfig{
				MaxFiles:      10,
				MaxFileSizeMB: 10,
			},
			Resources: &structs.Resources{
				CPU:      500,
				MemoryMB: -1,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	for _, task := range tasks {
		ctx := testDriverContexts(t, task)
		defer ctx.AllocDir.Destroy()
		d := NewQemuDriver(ctx.DriverCtx)

		if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
			t.Fatalf("Prestart faild: %v", err)
		}

		resp, err := d.Start(ctx.ExecCtx, task)
		if err == nil {
			resp.Handle.Kill()
			t.Fatalf("Should've failed")
		}

		msg := task.Config["msg"].(string)
		if !strings.Contains(err.Error(), msg) {
			t.Fatalf("Expecting '%v' in '%v'", msg, err)
		}
	}
}

func TestQemuDriverGetMonitorPath(t *testing.T) {
	shortPath := generateString(10)
	_, err := getMonitorPath(shortPath, "0")
	if err != nil {
		t.Fatal("Should not have returned an error")
	}

	longPath := generateString(legacyMaxMonitorPathLen + 100)
	_, err = getMonitorPath(longPath, "0")
	if err == nil {
		t.Fatal("Should have returned an error")
	}
	_, err = getMonitorPath(longPath, "1")
	if err != nil {
		t.Fatal("Should not have returned an error")
	}

	maxLengthPath := generateString(legacyMaxMonitorPathLen)
	_, err = getMonitorPath(maxLengthPath, "0")
	if err != nil {
		t.Fatal("Should not have returned an error")
	}
}
