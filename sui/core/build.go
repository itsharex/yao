package core

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"
	"github.com/yaoapp/kun/log"
	"golang.org/x/net/html"
)

var slotRe = regexp.MustCompile(`\[\{([^\}]+)\}\]`)
var cssRe = regexp.MustCompile(`([\.a-z0-9A-Z-:# ]+)\{`)
var langFuncRe = regexp.MustCompile(`L\s*\(\s*["'](.*?)["']\s*\)`)
var langAttrRe = regexp.MustCompile(`'::(.*?)'`)

// Build build the page
func (page *Page) Build(ctx *BuildContext, option *BuildOption) (*goquery.Document, []string, error) {
	// Create the context if not exists
	if ctx == nil {
		ctx = NewBuildContext(nil)
	}

	ctx.sequence++
	html, err := page.BuildHTML(option)
	if err != nil {
		ctx.warnings = append(ctx.warnings, err.Error())
	}

	doc, err := NewDocumentString(html)
	if err != nil {
		return nil, ctx.warnings, err
	}

	err = page.buildComponents(doc, ctx, option)
	if err != nil {
		return nil, ctx.warnings, err
	}

	// Scripts
	namespace := Namespace(page.Name, ctx.sequence)
	scripts, err := page.BuildScripts(ctx, option, "__page", namespace)
	if err != nil {
		return nil, ctx.warnings, err
	}

	// Styles
	styles, err := page.BuildStyles(ctx, option, "__page", namespace)
	if err != nil {
		return nil, ctx.warnings, err
	}

	// Append the scripts and styles
	ctx.scripts = append(ctx.scripts, scripts...)
	ctx.styles = append(ctx.styles, styles...)

	return doc, ctx.warnings, err
}

// BuildAsComponent build the page as component
func (page *Page) BuildAsComponent(sel *goquery.Selection, ctx *BuildContext, option *BuildOption) (string, error) {
	if page.parent == nil {
		return "", fmt.Errorf("The parent page is not set")
	}

	name, exists := sel.Attr("is")
	if !exists {
		return "", fmt.Errorf("The component tag must have an is attribute")
	}

	namespace := Namespace(name, ctx.sequence)
	component := ComponentName(name)
	attrs := []html.Attribute{
		{Key: "s:ns", Val: namespace},
		{Key: "s:cn", Val: component},
		{Key: "s:ready", Val: component + "()"},
	}

	ctx.sequence++
	var opt = *option
	opt.IgnoreDocument = true
	html, err := page.BuildHTML(&opt)
	if err != nil {
		return "", err
	}

	doc, err := NewDocumentStringWithWrapper(html)
	if err != nil {
		return "", err
	}

	body := doc.Selection.Find("body")
	if body.Length() > 1 {
		body.SetHtml("<div>" + html + "</div>")
	}

	// Scripts
	scripts, err := page.BuildScripts(ctx, &opt, component, namespace)
	if err != nil {
		return "", err
	}

	// Append the scripts
	ctx.scripts = append(ctx.scripts, scripts...)

	// Pass the component props
	first := body.Children().First()

	page.copyProps(ctx, sel, first, attrs...)
	page.buildComponents(doc, ctx, &opt)
	html, err = body.Html()
	if err != nil {
		return "", err
	}

	// Update the component
	data := Data{"$props": page.Attrs}
	html, _ = data.ReplaceUse(slotRe, html)
	sel.ReplaceWithHtml(html)
	return html, nil
}

func (page *Page) copyProps(ctx *BuildContext, from *goquery.Selection, to *goquery.Selection, extra ...html.Attribute) error {
	attrs := from.Get(0).Attr
	prefix := "s:prop"
	if page.Attrs == nil {
		page.Attrs = map[string]string{}
	}
	for _, attr := range attrs {
		if strings.HasPrefix(attr.Key, "s:") || attr.Key == "is" || attr.Key == "parsed" {
			continue
		}

		if strings.HasPrefix(attr.Key, "...[{") {
			data := Data{"$props": page.parent.Attrs}

			val, err := data.Exec(attr.Key[3:])
			if err != nil {
				ctx.warnings = append(ctx.warnings, err.Error())
				continue
			}

			switch value := val.(type) {
			case map[string]string:
				for key, value := range value {
					page.Attrs[key] = value
					key = fmt.Sprintf("%s:%s", prefix, key)
					to.SetAttr(key, value)

				}
			}
			continue
		}

		val := attr.Val
		if strings.HasPrefix(attr.Key, "...") {
			val = attr.Key[3:]
		}
		page.Attrs[attr.Key] = val
		key := fmt.Sprintf("%s:%s", prefix, attr.Key)
		to.SetAttr(key, val)
	}

	if len(extra) > 0 {
		for _, attr := range extra {
			to.SetAttr(attr.Key, attr.Val)
		}
	}

	return nil
}

func (page *Page) buildComponents(doc *goquery.Document, ctx *BuildContext, option *BuildOption) error {
	sui := SUIs[page.SuiID]
	if sui == nil {
		return fmt.Errorf("SUI %s not found", page.SuiID)
	}

	public := sui.GetPublic()
	tmpl, err := sui.GetTemplate(page.TemplateID)
	if err != nil {
		return err
	}

	doc.Find("*").Each(func(i int, sel *goquery.Selection) {
		// Get the translation

		name, has := sel.Attr("is")
		if !has {
			return
		}

		sel.SetAttr("parsed", "true")

		// Check if Just-In-Time Component ( "is" has variable )
		if ctx.isJitComponent(name) {
			sel.SetAttr("s:jit", "true")
			sel.SetAttr("s:root", public.Root)
			ctx.addJitComponent(name)
			return
		}

		ipage, err := tmpl.Page(name)
		if err != nil {
			sel.ReplaceWith(fmt.Sprintf("<!-- %s -->", err.Error()))
			log.Warn("Page %s/%s/%s: %s", page.SuiID, page.TemplateID, page.Route, err.Error())
			return
		}

		err = ipage.Load()
		if err != nil {
			sel.ReplaceWith(fmt.Sprintf("<!-- %s -->", err.Error()))
			log.Warn("Page %s/%s/%s: %s", page.SuiID, page.TemplateID, page.Route, err.Error())
			return
		}

		component := ipage.Get()
		component.parent = page
		component.BuildAsComponent(sel, ctx, option)
		return
	})

	return err
}

// BuildStyles build the styles for the page
func (page *Page) BuildStyles(ctx *BuildContext, option *BuildOption, component string, namespace string) ([]StyleNode, error) {
	styles := []StyleNode{}
	if page.Codes.CSS.Code == "" {
		return styles, nil
	}

	if _, has := ctx.styleUnique[component]; has {
		return styles, nil
	}
	ctx.styleUnique[component] = true

	code := page.Codes.CSS.Code
	// Replace the assets
	if !option.IgnoreAssetRoot {
		code = AssetsRe.ReplaceAllStringFunc(code, func(match string) string {
			return strings.ReplaceAll(match, "@assets", option.AssetRoot)
		})
	}

	if option.ComponentName != "" {
		code = cssRe.ReplaceAllStringFunc(code, func(css string) string {
			return fmt.Sprintf("[s\\:cn=%s] %s", option.ComponentName, css)
		})
		res, err := page.CompileCSS([]byte(code), option.StyleMinify)
		if err != nil {
			return styles, err
		}
		styles = append(styles, StyleNode{
			Namespace: namespace,
			Component: component,
			Source:    string(res),
			Parent:    "head",
			Attrs: []html.Attribute{
				{Key: "rel", Val: "stylesheet"},
				{Key: "type", Val: "text/css"},
			},
		})
		return styles, nil
	}

	res, err := page.CompileCSS([]byte(code), option.StyleMinify)
	if err != nil {
		return styles, err
	}
	styles = append(styles, StyleNode{
		Namespace: namespace,
		Component: component,
		Parent:    "head",
		Source:    string(res),
		Attrs: []html.Attribute{
			{Key: "rel", Val: "stylesheet"},
			{Key: "type", Val: "text/css"},
		},
	})

	return styles, nil
}

// BuildScripts build the scripts for the page
func (page *Page) BuildScripts(ctx *BuildContext, option *BuildOption, component string, namespace string) ([]ScriptNode, error) {

	scripts := []ScriptNode{}
	if page.Codes.JS.Code == "" && page.Codes.TS.Code == "" {
		return scripts, nil
	}
	if _, has := ctx.scriptUnique[component]; has {
		return scripts, nil
	}

	ctx.scriptUnique[component] = true
	var err error = nil
	var imports []string = nil
	var source []byte = nil
	if page.Codes.TS.Code != "" {
		source, imports, err = page.CompileTS([]byte(page.Codes.TS.Code), option.ScriptMinify)
		if err != nil {
			return nil, err
		}

	} else if page.Codes.JS.Code != "" {
		source, imports, err = page.CompileJS([]byte(page.Codes.JS.Code), option.ScriptMinify)
		if err != nil {
			return nil, err
		}

	}

	// Add the script
	if imports != nil {
		for _, src := range imports {
			scripts = append(scripts, ScriptNode{
				Namespace: namespace,
				Component: component,
				Parent:    "head",
				Attrs: []html.Attribute{
					{Key: "src", Val: fmt.Sprintf("%s/%s", option.AssetRoot, src)},
					{Key: "type", Val: "text/javascript"},
				}},
			)
		}
	}

	// Replace the assets
	if !option.IgnoreAssetRoot && source != nil {
		source = AssetsRe.ReplaceAllFunc(source, func(match []byte) []byte {
			return []byte(strings.ReplaceAll(string(match), "@assets", option.AssetRoot))
		})

		code := string(source)
		parent := "body"
		if component != "__page" {
			parent = "head"
			code = fmt.Sprintf("function %s(){\n%s\n}\n", component, addTabToEachLine(code))
		}

		scripts = append(scripts, ScriptNode{
			Namespace: namespace,
			Component: component,
			Source:    code,
			Parent:    parent,
			Attrs: []html.Attribute{
				{Key: "type", Val: "text/javascript"},
			},
		})
	}

	return scripts, nil
}

// BuildHTML build the html
func (page *Page) BuildHTML(option *BuildOption) (string, error) {

	html := string(page.Codes.HTML.Code)

	if option.WithWrapper {
		html = fmt.Sprintf("<body>%s</body>", html)
	}

	if !option.IgnoreDocument {
		html = string(page.Document)
		if page.Codes.HTML.Code != "" {
			html = strings.Replace(html, "{{ __page }}", page.Codes.HTML.Code, 1)
		}
	}

	if !option.IgnoreAssetRoot {
		html = strings.ReplaceAll(html, "@assets", option.AssetRoot)
	}

	res, err := page.CompileHTML([]byte(html), false)
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func addTabToEachLine(input string, prefix ...string) string {
	var lines []string

	space := "  "
	if len(prefix) > 0 {
		space = prefix[0]
	}

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		lineWithTab := space + line
		lines = append(lines, lineWithTab)
	}

	return strings.Join(lines, "\n")
}

func getScriptTranslation(code string, namespace string) []Translation {
	translations := []Translation{}
	matches := langFuncRe.FindAllStringSubmatch(code, -1)
	for i, match := range matches {
		translations = append(translations, Translation{
			Key:     fmt.Sprintf("%s_script_%d", namespace, i),
			Message: match[1],
			Type:    "script",
		})
	}
	return translations
}

func getNodeTranslation(sel *goquery.Selection, index int, namespace string) []Translation {

	translations := []Translation{}
	nodeType := sel.Get(0).Type
	switch nodeType {
	case html.ElementNode:

		// Get the translation
		if typ, has := sel.Attr("s:trans"); has {
			typ = strings.TrimSpace(typ)
			if typ == "" {
				typ = "html"
			}

			key := fmt.Sprintf("%s_index_%d", namespace, index)
			translations = append(translations, Translation{
				Key:     key,
				Message: strings.TrimSpace(sel.Text()),
				Type:    typ,
			})
			sel.SetAttr("s:trans-node", key)
			sel.RemoveAttr("s:trans")
		}

		// Attributes
		keys := map[string][]string{}
		has := false
		for i, attr := range sel.Get(0).Attr {

			keys[attr.Key] = []string{}

			// value="::attr"
			if strings.HasPrefix(attr.Val, "::") {
				key := fmt.Sprintf("%s_index_attr_%d_%d", namespace, index, i)
				translations = append(translations, Translation{
					Key:     fmt.Sprintf("%s_index_attr_%d_%d", namespace, index, i),
					Message: attr.Val[2:],
					Name:    attr.Key,
					Type:    "attr",
				})
				keys[attr.Key] = append(keys[attr.Key], key)
				has = true
			}

			// value="{{ 'key': '::value' }}"
			matches := langAttrRe.FindAllStringSubmatch(attr.Val, -1)
			if len(matches) > 0 {
				for j, match := range matches {
					key := fmt.Sprintf("%s_index_attr_%d_%d_%d", namespace, index, i, j)
					translations = append(translations, Translation{
						Key:     fmt.Sprintf("%s_index_attr_%d_%d_%d", namespace, index, i, j),
						Message: match[1],
						Name:    attr.Key,
						Type:    "attr",
					})
					keys[attr.Key] = append(keys[attr.Key], key)
					has = true
				}
			}
		}

		if has {
			raw, err := jsoniter.Marshal(keys)
			if err != nil {
				fmt.Println(color.RedString(err.Error()))
				break
			}
			sel.SetAttr("s:trans-attrs", string(raw))
			sel.RemoveAttr("s:trans")
		}

	case html.TextNode:
		if strings.HasPrefix(sel.Text(), "::") {
			key := fmt.Sprintf("%s_index_%d", namespace, index)
			translations = append(translations, Translation{
				Key:     fmt.Sprintf("%s_index_%d", namespace, index),
				Message: strings.TrimSpace(sel.Text()[2:]),
				Type:    "text",
			})
			sel.SetAttr("s:trans-node", key)
			sel.RemoveAttr("s:trans")
		}
	}

	return translations

}
