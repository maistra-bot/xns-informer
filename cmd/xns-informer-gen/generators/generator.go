package generators

import (
	"path"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

// TODO: Only generating a limite set of types for now.
var typesToGenerate = map[string][]string{
	"k8s.io/api/core/v1": {
		"ConfigMap",
		"Endpoints",
		"Pod",
		"Service",
	},
	"k8s.io/api/discovery/v1alpha1": {"EndpointSlices"},
	"k8s.io/api/networking/v1beta1": {"Ingress", "IngressClass"},
}

var headerText = []byte("// Code generated by xns-informer-gen. DO NOT EDIT.\n")

// TODO: This should be configurable via CustomArgs.
var pluralExceptions = map[string]string{"Endpoints": "Endpoints"}

const xnsinformersPkg = "github.com/maistra/xns-informer/pkg/informers"

type CustomArgs struct {
	ListersPackage   string
	InformersPackage string
}

type GroupVersion struct {
	Group   string
	Version string
}

// NameSystems returns the name system used by the generators in this package.
func NameSystems() namer.NameSystems {
	return namer.NameSystems{
		"public":             namer.NewPublicNamer(0),
		"private":            namer.NewPrivateNamer(0),
		"allLowercasePlural": namer.NewAllLowercasePluralNamer(pluralExceptions),
		"publicPlural":       namer.NewPublicPluralNamer(pluralExceptions),
	}
}

// DefaultNameSystem returns the default name system for ordering the types to be
// processed by the generators in this package.
func DefaultNameSystem() string {
	return "public"
}

func Packages(c *generator.Context, args *args.GeneratorArgs) generator.Packages {
	customArgs, ok := args.CustomArgs.(*CustomArgs)
	if !ok {
		klog.Fatal("Invalid type for CustomArgs")
	}

	var packages []generator.Package
	groupVersions := make(map[string][]string)

	for _, inputDir := range args.InputDirs {
		p := c.Universe.Package(inputDir)

		gv := GroupVersion{}
		parts := strings.Split(p.Path, "/")
		gv.Group = parts[len(parts)-2]
		gv.Version = parts[len(parts)-1]

		if _, ok := groupVersions[gv.Group]; !ok {
			groupVersions[gv.Group] = make([]string, 0)
		}

		groupVersions[gv.Group] = append(groupVersions[gv.Group], gv.Version)

		listersPkg := path.Join(customArgs.ListersPackage, gv.Group, gv.Version)
		informersPkg := path.Join(customArgs.InformersPackage, gv.Group, gv.Version)
		outputPath := filepath.Join(gv.Group, gv.Version)

		packages = append(packages, &generator.DefaultPackage{
			PackageName: gv.Version,
			PackagePath: outputPath,
			HeaderText:  headerText,

			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				packageTypes, ok := typesToGenerate[p.Path]
				if !ok {
					return false
				}

				for i := range packageTypes {
					if packageTypes[i] == t.Name.Name {
						return true
					}
				}

				return false
			},

			GeneratorFunc: func(c *generator.Context) []generator.Generator {
				var generators []generator.Generator

				generators = append(generators, &versionInterfaceGenerator{
					DefaultGen: generator.DefaultGen{
						OptionalName: "interface",
					},
					informersPackage: informersPkg,
					groupVersion:     gv,
					types:            c.Order,
				})

				for _, t := range c.Order {
					generators = append(generators, &listerGenerator{
						DefaultGen: generator.DefaultGen{
							OptionalName: strings.ToLower(t.Name.Name),
						},
						imports:          generator.NewImportTracker(),
						listersPackage:   listersPkg,
						informersPackage: informersPkg,
						groupVersion:     gv,
						typeToGenerate:   t,
					})
				}
				return generators
			},
		})
	}

	for group, versions := range groupVersions {
		outputPackage := path.Join(args.OutputPackagePath, group)
		packages = append(packages, &generator.DefaultPackage{
			PackageName: group,
			PackagePath: group,
			HeaderText:  headerText,

			GeneratorFunc: func(c *generator.Context) []generator.Generator {
				var generators []generator.Generator
				generators = append(generators, &groupInterfaceGenerator{
					DefaultGen: generator.DefaultGen{
						OptionalName: "interface",
					},
					group:         group,
					versions:      versions,
					outputPackage: outputPackage,
					imports:       generator.NewImportTracker(),
				})
				return generators
			},
		})
	}

	return packages
}