package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DocumentationInfo holds comprehensive package documentation
type DocumentationInfo struct {
	Name        string
	Description string
	Files       []string
	Types       []TypeInfo
	Functions   []FunctionInfo
	Interfaces  []InterfaceInfo
}

// TypeInfo represents documentation for a type
type TypeInfo struct {
	Name        string
	Description string
	Methods     []FunctionInfo
}

// FunctionInfo represents documentation for a function
type FunctionInfo struct {
	Name        string
	Description string
	Signature   string
}

// InterfaceInfo represents documentation for an interface
type InterfaceInfo struct {
	Name        string
	Description string
	Methods     []FunctionInfo
}

// parseGoFile parses a single Go file and extracts documentation
func parseGoFile(filePath string) (*DocumentationInfo, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Create documentation package
	pkg := &ast.Package{
		Name:  file.Name.Name,
		Files: map[string]*ast.File{filePath: file},
	}

	// Use go/doc to extract documentation
	docPkg := doc.New(pkg, "./", doc.AllDecls)

	docInfo := &DocumentationInfo{
		Name:        docPkg.Name,
		Description: docPkg.Doc,
		Files:       []string{filePath},
	}

	// Parse types
	for _, t := range docPkg.Types {
		// Check if it's an interface by inspecting its specification
		isInterface := false
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == t.Name {
						if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
							isInterface = true
							break
						}
					}
				}
				if isInterface {
					break
				}
			}
		}

		if isInterface {
			interfaceInfo := InterfaceInfo{
				Name:        t.Name,
				Description: t.Doc,
			}

			// Parse interface methods
			for _, method := range t.Methods {
				interfaceInfo.Methods = append(interfaceInfo.Methods, FunctionInfo{
					Name:        method.Name,
					Description: method.Doc,
					Signature:   method.Decl.Name.Name, // Simplified signature
				})
			}

			docInfo.Interfaces = append(docInfo.Interfaces, interfaceInfo)
		} else {
			typeInfo := TypeInfo{
				Name:        t.Name,
				Description: t.Doc,
			}

			// Parse methods
			for _, method := range t.Methods {
				typeInfo.Methods = append(typeInfo.Methods, FunctionInfo{
					Name:        method.Name,
					Description: method.Doc,
					Signature:   method.Decl.Name.Name, // Simplified signature
				})
			}

			docInfo.Types = append(docInfo.Types, typeInfo)
		}
	}

	// Parse functions
	for _, f := range docPkg.Funcs {
		docInfo.Functions = append(docInfo.Functions, FunctionInfo{
			Name:        f.Name,
			Description: f.Doc,
			Signature:   f.Decl.Name.Name, // Simplified signature
		})
	}

	return docInfo, nil
}

// generateDocumentationHTML generates an HTML documentation page
func generateDocumentationHTML(docInfo *DocumentationInfo) (string, error) {
	const docTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>Go Basic Blockchain Documentation - {{ .Name }}</title>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; max-width: 1200px; margin: 0 auto; padding: 20px; }
		h1, h2 { color: #333; }
		.section { margin-bottom: 30px; }
		.description { color: #666; }
		.code { background-color: #f4f4f4; padding: 10px; border-radius: 5px; }
	</style>
</head>
<body>
	<h1>Package: {{ .Name }}</h1>
	
	<div class="section">
		<h2>Overview</h2>
		<p class="description">{{ .Description }}</p>
	</div>

	{{ if .Types }}
	<div class="section">
		<h2>Types</h2>
		{{ range .Types }}
			<div>
				<h3>{{ .Name }}</h3>
				<p class="description">{{ .Description }}</p>
				{{ if .Methods }}
					<h4>Methods</h4>
					<ul>
					{{ range .Methods }}
						<li>
							<strong>{{ .Name }}</strong>
							<p class="description">{{ .Description }}</p>
						</li>
					{{ end }}
					</ul>
				{{ end }}
			</div>
		{{ end }}
	</div>
	{{ end }}

	{{ if .Functions }}
	<div class="section">
		<h2>Functions</h2>
		{{ range .Functions }}
			<div>
				<h3>{{ .Name }}</h3>
				<p class="description">{{ .Description }}</p>
			</div>
		{{ end }}
	</div>
	{{ end }}

	{{ if .Interfaces }}
	<div class="section">
		<h2>Interfaces</h2>
		{{ range .Interfaces }}
			<div>
				<h3>{{ .Name }}</h3>
				<p class="description">{{ .Description }}</p>
				{{ if .Methods }}
					<h4>Methods</h4>
					<ul>
					{{ range .Methods }}
						<li>
							<strong>{{ .Name }}</strong>
							<p class="description">{{ .Description }}</p>
						</li>
					{{ end }}
					</ul>
				{{ end }}
			</div>
		{{ end }}
	</div>
	{{ end }}
</body>
</html>
`

	tmpl, err := template.New("documentation").Parse(docTemplate)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	err = tmpl.Execute(&output, docInfo)
	if err != nil {
		return "", err
	}

	return output.String(), nil
}

func main() {
	// Create docs directory if it doesn't exist
	docsDir := "docs"
	err := os.MkdirAll(docsDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create docs directory: %v", err)
	}

	// Collect all Go files in the SDK directory
	sdkFiles := []string{}
	err = filepath.Walk("sdk", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.Contains(path, "_test.go") {
			sdkFiles = append(sdkFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to walk SDK directory: %v", err)
	}

	// Sort files to ensure consistent documentation generation
	sort.Strings(sdkFiles)

	// Generate documentation for each file
	for _, file := range sdkFiles {
		docInfo, err := parseGoFile(file)
		if err != nil {
			log.Printf("Error parsing %s: %v", file, err)
			continue
		}

		// Generate HTML documentation
		htmlDoc, err := generateDocumentationHTML(docInfo)
		if err != nil {
			log.Printf("Error generating HTML for %s: %v", file, err)
			continue
		}

		// Write HTML documentation
		outputPath := filepath.Join(docsDir, strings.TrimSuffix(filepath.Base(file), ".go")+".html")
		err = os.WriteFile(outputPath, []byte(htmlDoc), 0644)
		if err != nil {
			log.Printf("Error writing documentation for %s: %v", file, err)
			continue
		}

		fmt.Printf("Generated documentation for %s\n", file)
	}

	// Create an index.html that links to all generated documentation
	err = generateIndexHTML(sdkFiles)
	if err != nil {
		log.Fatalf("Failed to generate index.html: %v", err)
	}
}

func generateIndexHTML(files []string) error {
	const indexTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>Go Basic Blockchain SDK Documentation</title>
	<style>
		body { font-family: Arial, sans-serif; line-height: 1.6; max-width: 800px; margin: 0 auto; padding: 20px; }
		h1 { color: #333; }
		ul { list-style-type: none; padding: 0; }
		li { margin-bottom: 10px; }
		a { color: #0066cc; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<h1>Go Basic Blockchain SDK Documentation</h1>
	<ul>
		{{range .}}
		<li><a href="{{.}}">{{.}}</a></li>
		{{end}}
	</ul>
</body>
</html>
`

	// Extract just the filenames
	var fileNames []string
	for _, file := range files {
		fileNames = append(fileNames, strings.TrimSuffix(filepath.Base(file), ".go")+".html")
	}

	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		return err
	}

	indexFile, err := os.Create(filepath.Join("docs", "index.html"))
	if err != nil {
		return err
	}
	defer indexFile.Close()

	err = tmpl.Execute(indexFile, fileNames)
	if err != nil {
		return err
	}

	fmt.Println("Generated index.html for documentation")
	return nil
}
