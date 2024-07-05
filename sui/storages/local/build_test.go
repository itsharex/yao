package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yaoapp/gou/application"
	"github.com/yaoapp/yao/sui/core"
)

func TestTemplateBuild(t *testing.T) {
	tests := prepare(t)
	defer clean()

	tmpl, err := tests.Test.GetTemplate("advanced")
	if err != nil {
		t.Fatalf("GetTemplate error: %v", err)
	}

	root := application.App.Root()
	public := tmpl.(*Template).local.GetPublic()
	path := filepath.Join(root, "public", public.Root)

	// Remove files and directories in Public directory if exists
	err = os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("RemoveAll error: %v", err)
	}

	err = tmpl.Build(&core.BuildOption{SSR: true, ExecScripts: true})
	if err != nil {
		t.Fatalf("Components error: %v", err)
	}

	index := "/index.sui"

	// Check SUI
	assert.FileExists(t, filepath.Join(path, index))
	content, err := os.ReadFile(filepath.Join(path, index))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	assert.Contains(t, string(content), "body")
	assert.Contains(t, string(content), `<script src="/unit-test/assets/js/import.js"></script>`)
	assert.Contains(t, string(content), `<script name="config" type="json">`)
	assert.Contains(t, string(content), `<script name="data" type="json">`)
	assert.Contains(t, string(content), `<script name="global" type="json">`)

}

func TestTemplateBuildAsComponent(t *testing.T) {
	tests := prepare(t)
	defer clean()

	tmpl, err := tests.Web.GetTemplate("default")
	if err != nil {
		t.Fatalf("GetTemplate error: %v", err)
	}

	root := application.App.Root()
	public := tmpl.(*Template).local.GetPublic()
	path := filepath.Join(root, "public", public.Root)

	// Remove files and directories in Public directory if exists
	err = os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("RemoveAll error: %v", err)
	}

	err = tmpl.Build(&core.BuildOption{SSR: true})
	if err != nil {
		t.Fatalf("Components error: %v", err)
	}

	cselect := "/flowbite/components/edit/select.jit"
	cinput := "/flowbite/components/edit/input.jit"

	// Check JIT
	assert.FileExists(t, filepath.Join(path, cselect))
	assert.FileExists(t, filepath.Join(path, cinput))

	content, err := os.ReadFile(filepath.Join(path, cselect))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	assert.NotContains(t, string(content), "body")
	assert.NotContains(t, string(content), `<script name="config" type="json">`)
	assert.NotContains(t, string(content), `<script name="data" type="json">`)
	assert.NotContains(t, string(content), `<script name="global" type="json">`)
	assert.Contains(t, string(content), "function Init()")
	assert.Contains(t, string(content), `type="flowbite-edit-select"`)
}

func TestPageBuild(t *testing.T) {
	tests := prepare(t)
	defer clean()

	tmpl, err := tests.Test.GetTemplate("advanced")
	if err != nil {
		t.Fatalf("GetTemplate error: %v", err)
	}

	root := application.App.Root()
	public := tmpl.(*Template).local.GetPublic()
	path := filepath.Join(root, "public", public.Root)

	// Remove files and directories in Public directory if exists
	err = os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("RemoveAll error: %v", err)
	}

	page, err := tmpl.Page("/index")
	if err != nil {
		t.Fatalf("Page error: %v", err)
	}

	err = page.Build(nil, &core.BuildOption{SSR: true})
	if err != nil {
		t.Fatalf("Page Build error: %v", err)
	}
	index := "/index.sui"

	// Check SUI
	assert.FileExists(t, filepath.Join(path, index))

	content, err := os.ReadFile(filepath.Join(path, index))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	assert.Contains(t, string(content), "body")
	assert.Contains(t, string(content), `<script src="/unit-test/assets/js/import.js"></script>`)
	assert.Contains(t, string(content), `<script name="config" type="json">`)
	assert.Contains(t, string(content), `<script name="data" type="json">`)
	assert.Contains(t, string(content), `<script name="global" type="json">`)
}

func TestPageBuildAsComponent(t *testing.T) {
	tests := prepare(t)
	defer clean()

	tmpl, err := tests.Web.GetTemplate("default")
	if err != nil {
		t.Fatalf("GetTemplate error: %v", err)
	}

	root := application.App.Root()
	public := tmpl.(*Template).local.GetPublic()
	path := filepath.Join(root, "public", public.Root)

	// Remove files and directories in Public directory if exists
	err = os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("RemoveAll error: %v", err)
	}

	page, err := tmpl.Page("/[form]")
	if err != nil {
		t.Fatalf("Page error: %v", err)
	}

	err = page.Build(nil, &core.BuildOption{SSR: true})
	if err != nil {
		t.Fatalf("Components error: %v", err)
	}

	cselect := "/flowbite/components/edit/select.jit"
	cinput := "/flowbite/components/edit/input.jit"

	// Check JIT
	assert.FileExists(t, filepath.Join(path, cselect))
	assert.FileExists(t, filepath.Join(path, cinput))

	content, err := os.ReadFile(filepath.Join(path, cselect))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	assert.NotContains(t, string(content), "body")
	assert.NotContains(t, string(content), `<script name="config" type="json">`)
	assert.NotContains(t, string(content), `<script name="data" type="json">`)
	assert.NotContains(t, string(content), `<script name="global" type="json">`)
	assert.Contains(t, string(content), "function Init()")
	assert.Contains(t, string(content), `type="flowbite-edit-select"`)
}
