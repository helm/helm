# Three Level Dependent Chart

This chart is for testing the processing of multi-level dependencies.

Consists of the following charts:

- Library Chart
- App Chart (Uses Library Chart as dependecy, 2x: app1/app2)
- Umbrella Chart (Has all the app charts as dependencies)

The precendence is as follows: `library < app < umbrella`

Catches two use-cases:

- app overwriting library (app2)
- umbrella overwriting app and library (app1)
