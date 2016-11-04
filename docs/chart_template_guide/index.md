# The Chart Template Developer's Guide

This guide provides an introduction to Helm's chart templates, with emphasis on
the template language.

Templates generate manifest files, which are YAML-formatted resource descriptions
that Kubernetes can understand. We'll look at how templates are structured,
how they can be used, how to write Go templates, and how to debug your work.

This guide focuses on the following concepts:

- The Helm template language
- Using values
- Techniques for working with templates

This guide is oriented toward learning the ins and outs of the Helm template language. Other guides provide introductory material, examples, and best practices.