package composition

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
	"strings"
	"testing"
	"time"
)

var integratedTestHtml = `<html>
  <head>
    <link uic-remove rel="stylesheet" type="text/css" href="testing.css"/>
    <link rel="stylesheet" type="text/css" href="special.css"/>
    <script type="text/uic-meta">
      {
       "foo": "bar",
       "boo": "bazz",
       "categories": ["animal", "human"]
      }
    </script>
    <script uic-remove>
      va xzw = "some test code""
    </script>
    <script src="myScript.js"></script>
  </head>
  <body>
    <ul uic-remove>
      <!-- A Navigation for testing -->
    </ul>
    <uic-fragment name="headline">
      <h1>This is a headline</h1>
    </uic-fragment>
    <uic-fragment name="content">
      Bli Bla blub
      <uic-include src="example.com/foo#content" timeout="42000" required="true"/>
      <uic-include src="example.com/optional#content" timeout="100" required="false"/>
      <div uic-remove>
         Some element for testing
      </div>
      <hr/>
      Bli Bla blub
    </uic-fragment>
    <uic-tail>
      <!-- some script tags to insert at the end -->
      <script src="foo.js"></script>
      <script src="bar.js"></script>
      <script uic-remove src="demo.js"></script>
    </uic-tail>
  </body>
</html>
`

var integratedTestHtmlExpectedMeta = map[string]interface{}{
	"foo":        "bar",
	"boo":        "bazz",
	"categories": []interface{}{"animal", "human"},
}

var integratedTestHtmlExpectedHead = `
    <link rel="stylesheet" type="text/css" href="special.css"/>
    <script src="myScript.js"></script>`

var integratedTestHtmlExpectedHeadline = `<h1>This is a headline</h1>`

var integratedTestHtmlExpectedContent = `
      Bli Bla blub
      §[> example.com/foo#content]§
      §[> example.com/optional#content]§
      <hr/>
      Bli Bla blub`

var integratedTestHtmlExpectedTail = `
      <!-- some script tags to insert at the end -->
      <script src="foo.js"></script>
      <script src="bar.js"></script>`

func Test_HtmlContentParser_LoadEmptyContent(t *testing.T) {
	a := assert.New(t)

	in := strings.NewReader(`<html>
  <head>
  </head>
  <body>
  </body>
</html>
`)
	c := NewMemoryContent()
	parser := &HtmlContentParser{}
	err := parser.Parse(c, in)
	a.NoError(err)

	a.Equal(0, len(c.Body()))
	a.Equal(0, len(c.Meta()))
	a.Equal(0, len(c.RequiredContent()))
	a.Nil(c.Head())
	a.Nil(c.Tail())
}

func Test_HtmlContentParser_parseHead(t *testing.T) {
	a := assert.New(t)

	parser := &HtmlContentParser{}
	z := html.NewTokenizer(bytes.NewBufferString(`<head>
  <div uic-remove>
    <script>
    sdcsdc
    </script>
  </div>
  <xx/> 
  <foo>xxx</foo>
  <div uic-remove>
    <script>
    sdcsdc
    </script>
  </div> 
  <bar>xxx</bar>
  <script type="text/uic-meta">
      {
       "foo": "bar"
      }
  </script>
  <div uic-remove>
    <script>
    sdcsdc
    </script>
  </div>
</head>
`))

	z.Next() // At <head ..
	c := NewMemoryContent()
	err := parser.parseHead(z, c)
	a.NoError(err)

	eqFragment(t, "<xx/><foo>xxx</foo><bar>xxx</bar>", c.Head())
	a.Equal("bar", c.Meta()["foo"])
}

func Test_HtmlContentParser_parseBody(t *testing.T) {
	a := assert.New(t)

	parser := &HtmlContentParser{}
	z := html.NewTokenizer(bytes.NewBufferString(`<body some="attribute">
    <h1>Default Fragment Content</h1><br>
    <ul uic-remove>
      <!-- A Navigation for testing -->
    </ul>
    <uic-fragment name="headline">
      <h1>Headline</h1>
      <uic-include src="example.com/optional#content" timeout="100" required="false"/>
    </uic-fragment>
    <uic-fragment name="content">
      some content
      <uic-include src="example.com/foo#content" timeout="42000" required="true"/>
      <uic-include src="example.com/optional#content" timeout="100" required="false"/>
    </uic-fragment>
    <uic-tail>
      <!-- tail -->
      <uic-include src="example.com/tail" timeout="100" required="false"/>
    </uic-tail>
  </body>`))

	z.Next() // At <body ..
	c := NewMemoryContent()
	err := parser.parseBody(z, c)
	a.NoError(err)

	a.Equal(3, len(c.Body()))
	eqFragment(t, "<h1>Default Fragment Content</h1><br>", c.Body()[""])
	eqFragment(t, `<h1>Headline</h1> §[> example.com/optional#content]§`, c.Body()["headline"])
	eqFragment(t, `some content §[> example.com/foo#content]§ §[> example.com/optional#content]§`, c.Body()["content"])
	eqFragment(t, "<!-- tail -->§[> example.com/tail]§", c.Tail())

	eqFragment(t, `some="attribute"`, c.BodyAttributes())

	a.Equal(3, len(c.RequiredContent()))
	a.Equal(&FetchDefinition{
		URL:      "example.com/foo",
		Timeout:  time.Millisecond * 42000,
		Required: true,
	}, c.requiredContent["example.com/foo"])

	a.Equal(&FetchDefinition{
		URL:      "example.com/optional",
		Timeout:  time.Millisecond * 100,
		Required: false,
	}, c.requiredContent["example.com/optional"])
	a.Equal(&FetchDefinition{
		URL:      "example.com/tail",
		Timeout:  time.Millisecond * 100,
		Required: false,
	}, c.requiredContent["example.com/tail"])
}

func Test_HtmlContentParser_parseBody_OnlyDefaultFragment(t *testing.T) {
	a := assert.New(t)

	parser := &HtmlContentParser{}
	z := html.NewTokenizer(bytes.NewBufferString(`<body>
    <h1>Default Fragment Content</h1><br>
    <uic-include src="example.com/foo#content" timeout="42000" required="true"/>
  </body>`))

	z.Next() // At <body ..
	c := NewMemoryContent()
	err := parser.parseBody(z, c)
	a.NoError(err)

	a.Equal(1, len(c.Body()))
	eqFragment(t, "<h1>Default Fragment Content</h1><br> §[> example.com/foo#content]§", c.Body()[""])

	a.Equal(1, len(c.RequiredContent()))
	a.Equal(&FetchDefinition{
		URL:      "example.com/foo",
		Timeout:  time.Millisecond * 42000,
		Required: true,
	}, c.requiredContent["example.com/foo"])
}

func Test_HtmlContentParser_parseBody_DefaultFragmentOverwritten(t *testing.T) {
	a := assert.New(t)

	parser := &HtmlContentParser{}
	z := html.NewTokenizer(bytes.NewBufferString(`<body>
    <h1>Default Fragment Content</h1><br>
    <uic-fragment>
      Overwritten
    </uic-fragment>
  </body>`))

	z.Next() // At <body ..
	c := NewMemoryContent()
	err := parser.parseBody(z, c)
	a.NoError(err)

	a.Equal(1, len(c.Body()))
	eqFragment(t, "Overwritten", c.Body()[""])
}

func Test_HtmlContentParser_parseHead_JsonError(t *testing.T) {
	a := assert.New(t)

	parser := &HtmlContentParser{}
	z := html.NewTokenizer(bytes.NewBufferString(`
<script type="text/uic-meta">
      {
</script>
`))

	c := NewMemoryContent()
	err := parser.parseHead(z, c)

	a.Error(err)
	a.Contains(err.Error(), "error while parsing json from meta json")
}

func Test_HtmlContentParser_parseFragment(t *testing.T) {
	a := assert.New(t)

	z := html.NewTokenizer(bytes.NewBufferString(`<uic-fragment name="content">
      Bli Bla blub
      <br>
      <uic-include src="example.com/foo#content" timeout="42000" required="true"/>
      <uic-include src="example.com/optional#content" timeout="100" required="false"/>
      <div uic-remove>
         <br>
         Some element for testing
      </div>
      <hr/>     
      Bli Bla §[ aVariable ]§ blub
    </uic-fragment><testend>`))

	z.Next() // At <uic-fragment name ..
	f, deps, err := parseFragment(z)
	a.NoError(err)

	a.Equal(2, len(deps))
	a.Equal(&FetchDefinition{
		URL:      "example.com/foo",
		Timeout:  time.Millisecond * 42000,
		Required: true,
	}, deps[0])

	a.Equal(&FetchDefinition{
		URL:      "example.com/optional",
		Timeout:  time.Millisecond * 100,
		Required: false,
	}, deps[1])

	sFragment := f.(StringFragment)
	expected := `Bli Bla blub
      <br>
      §[> example.com/foo#content]§
      §[> example.com/optional#content]§
      <hr/>
      Bli Bla §[ aVariable ]§ blub`
	eqFragment(t, expected, sFragment)

	z.Next()
	endTag, _ := z.TagName()
	a.Equal("testend", string(endTag))
}

// Regression test: to ensure, that escaped entities in attributes do not lead to a problem.
func Test_HtmlContentParser_parseFragment_EntityAttribute(t *testing.T) {
	a := assert.New(t)

	testHtml := `<a style="text-decoration: none" href="/produktkatalog?&amp;page=91">`
	in := strings.NewReader(`<html><head>` + testHtml + `</head><body>` +
		testHtml + `<uic-fragment name="content">` + testHtml + `</uic-fragment></body></html>`)

	c := NewMemoryContent()
	parser := &HtmlContentParser{}
	err := parser.Parse(c, in)
	a.NoError(err)

	eqFragment(t, testHtml, c.Head())
	eqFragment(t, testHtml, c.Body()[""])
	eqFragment(t, testHtml, c.Body()["content"])
}

func Test_HtmlContentParser_parseMetaJson(t *testing.T) {
	a := assert.New(t)

	z := html.NewTokenizer(bytes.NewBufferString(`<script type="text/uic-meta">
      {
       "foo": "bar",
       "boo": "bazz",
       "categories": ["animal", "human"]
      }
    </script>`))

	z.Next() // At <script ..
	c := NewMemoryContent()
	err := parseMetaJson(z, c)
	a.NoError(err)

	a.Equal("bar", c.Meta()["foo"])
}

func Test_HtmlContentParser_parseMetaJson_Errors(t *testing.T) {
	a := assert.New(t)

	testCases := []struct {
		html      string
		errorText string
	}{
		{
			html:      `<script type="text/uic-meta"></script>`,
			errorText: "expected text node for meta",
		},
		{
			html:      `<script type="text/uic-meta">{"sdc":</script>`,
			errorText: "error while parsing json from meta json",
		},
		{
			html:      `<script type="text/uic-meta">{}`,
			errorText: "Tag not properly ended",
		},
	}

	for _, test := range testCases {
		z := html.NewTokenizer(bytes.NewBufferString(test.html))
		z.Next() // At <script ..
		err := parseMetaJson(z, NewMemoryContent())

		a.Error(err)
		a.Contains(err.Error(), test.errorText)
	}
}

func Test_HtmlContentParser_skipSubtreeIfUicRemove(t *testing.T) {
	a := assert.New(t)

	z := html.NewTokenizer(bytes.NewBufferString(`<a><b uic-remove>
    sdcsdc
    <hr/>
    <br>
    <img src="http://foo">
    <foo>xxx</foo>
    <br/>
</b></a>`))

	z.Next()
	tt := z.Next() // at b
	attrs := readAttributes(z, make([]html.Attribute, 0, 10))
	skipped := skipSubtreeIfUicRemove(z, tt, "b", attrs)

	a.True(skipped)
	token := z.Next()
	a.Equal(html.EndTagToken, token)
	tag, _ := z.TagName()
	a.Equal("a", string(tag))
}

func Test_joinAttrs(t *testing.T) {
	a := assert.New(t)
	a.Equal(``, joinAttrs([]html.Attribute{}))
	a.Equal(`some="attribute"`, joinAttrs([]html.Attribute{{Key: "some", Val: "attribute"}}))
	a.Equal(`a="b" some="attribute"`, joinAttrs([]html.Attribute{{Key: "a", Val: "b"}, {Key: "some", Val: "attribute"}}))
	a.Equal(`a="--&#34;--"`, joinAttrs([]html.Attribute{{Key: "a", Val: `--"--`}}))
	a.Equal(`ns:a="b"`, joinAttrs([]html.Attribute{{Namespace: "ns", Key: "a", Val: "b"}}))
}

func eqFragment(t *testing.T, expected string, f Fragment) {
	if f == nil {
		t.Error("Fragment is nil, but expected:", expected)
		return
	}
	sf := f.(StringFragment)
	sfStripped := strings.Replace(string(sf), " ", "", -1)
	sfStripped = strings.Replace(string(sfStripped), "\n", "", -1)
	expectedStripped := strings.Replace(expected, " ", "", -1)
	expectedStripped = strings.Replace(expectedStripped, "\n", "", -1)

	if expectedStripped != sfStripped {
		t.Error("Fragment is not equal: \nexpected: ", expected, "\nactual:  ", sf)
	}
}
