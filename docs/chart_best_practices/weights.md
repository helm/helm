# Chart and template weights

This part of the Best Practices Guide discusses how to manipulate the creation order of kubernetes resources

The weight of an object can be given at two levels:

- Chart: It can be defined in `Chart.yaml`. All templates directly defined in this chart is affected. It takes precedence over template weight.
- Template: A `helm.sh/order-weight` annotation shall be declared in the uppermost metadata section of a kubernetes object.

## Precedence

Every object will get a tuple value of (chart weight, object weight). Default is 0 for both.
Chart weight takes precedence over individual objects. Higher weight means it will be handled earlier.

## Scope

Weights have a global scope, so if subchart has higher weight then it will be handled sooner regardless of the inclusion order.

## How it works

Without the `--wait` flag it works like kind ordering - all objects will be sent to apiserver in one big manifest and objects are sorted accordingly (sortByWeight(sortByKind(templates))).

With the `--wait` flag equally weighted objects are grouped together, and these groups are then installed one-by-one. Helm waits for each group's resources individually for the given `--timeout` period.
