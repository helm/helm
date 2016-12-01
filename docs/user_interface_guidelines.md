# Helm User Interface Guidelines

This document specifies how to write build a consistent and useable command line client (CLI) that follows the conventions already established in the Helm project.

## The Goal Is Obvious

The goal of a good user interface is to make it _obvious_ to the user. This is achieved in three ways:

- Follow common patterns: Use similar conventions to other tools, with preference toward popular or ubiquitous tools.
- Apply consistent patterns: Establish patterns, conventions, and vocabulary. Then stick to them.
- Be mnemonic: Make it easy to remember how to get things done. Mnemonics are as much about creativity as they are about consistency.

This guide enumerates patterns and conventions that we use, and explains the rationale for many of Helm's choices.

## The Prime Directive of Helm Usability


The primary rule of usability is this: **User first, shell second**. All interactions are to be designed first for the human operator, and secondarily for shell scripting.

As a result...

1. The default output of any command should be intended for a human
2. Every command should be thoroughly documented
3. The simplest syntax is the common user case. In other words, the defaults for all options should be the values that are best for a human user.

It is far better to require a shell script to supply dozens of options than to require the user to type long sequences just to get a human-friendly view of the data.


## The Heritage of the UNIX Command

The first principle of developing a command-line tool is to pay due respects to our heritage. UNIX commands have a long, but dynamic, history. Helm utilizes many of the strategies that have evolved out of this history.

In the world of the command line, a program typically has three user-facing parts: The invocation, the runtime activity, and the output. In **the invocation**, the user enters (types) a full command (plus options and arguments), and then presses the Enter key to execute the command.

In **the runtime activity**, the program may accept input (via STDIN and prompts) and may send output (via STDOUT or STDERR).

When the program completes execution, one piece of output is always generated: **The exit code**. Many user environments do not display this. However, it is a fundamental component of interactivity with command line tools. It serves the purpose of alerting the environment as to final state of a program's execution.

## The CLI Invocation

In the traditional CLI, there are three major functional parts:

```
COMMAND [OPTIONS] [ARGUMENTS]
```

- The command: This part identifies the program (or function of a program) that is to be run. Sometimes the command portion is broken into sections and subcommands.
- Options: As the name implies, sometimes it is desirable to pass _optional information_ into a command for the purpose of better instructing the program about what to do. Following the UNIX convention, options are passed in using the _flag syntax_ (`-s` and `--long`).
- Arguments: Some commands _require_ certain pieces of information before they can complete their task. The _de facto_ UNIX convention (though not universally applied) is to provide this required information as arguments, which are typically a space-separated series of strings.

The following subsections deals in more detail with each of the three parts of an invocation.

### Commands

In many of the early command-line environments, commands were single words or mnemonics: `ls`, `find`, `ps`, `rm`. And these tools all followed the simple mantra that 'a tool should do one thing exceedingly well'. As programs became more sophisticated, the "one thing well" mantra approached its limits.

Some "programs" began to require multiple programs. For example, the SSH (Secure SHell) "program" is a suite of related command line programs: `ssh`, `ssh-add` `ssh-agent`, `ssh-keygen`, `sshd`, `scp`, `sftp`, and so on.

This pattern of a suite of related programs introduced a number of problems, not the least of which was that multiple separate binaries must be maintained and synchronized on the same system. One way of solving the problem is to provide a top-level tool with multiple "subcommands": fixed arguments (constants) that the program uses as a cue to execute a specific code path.

Many popular tools now use this method. Here are a few examples:

- `git pull origin master`: COMMAND SUBCOMMAND ARG1 ARG2
- `gem install foo`: COMMAND SUBCOMMAND ARG1

Predominantly (though not always) the subcommand is a verb, so the pattern is _noun verb_. Common exceptions to this rule include `version` and `status` commands: `git status`, `git version`.

Another emerging pattern is to use categories in conjunction with subcommands:

- `git stash pop`: COMMAND CATEGORY SUBCOMMAND
- `mix deps.get`: COMMAND CATEGORY.SUBCOMMAND
- `deis domains:list`: COMMAND CATEGORY:SUBCOMMAND
- `gem environment platform`: COMMAND CATEGORY SUBCOMMAND

The predominant, though not universal, grammatical structure of the command-category-subcommand pattern is _noun noun verb_.

There are many cases in which command-subcommand and command-category-subcommand patterns coexist in the same program. In fact, all programs listed above mix the two. But the ways in which they are mixed vary greatly, and are often not even internally consistent (viz. `git`).

Helm allows mixing the two accord to the following principles:

1. Primary tasks (things users do "most of the time") are subcommands: `helm install`, `helm list`.
2. Less frequent tasks are grouped into categories, where a category represents the conceptual bond between all subcommands: `helm repo add`, `helm repo list`, `helm repo delete`, `helm repo update`.
3. When a user invokes `COMMAND CATEGORY` with no subcommand, the program should print a sensible default listing or print help text specific to that category.

**Note:** Categories are occasionally confused with arguments. An argument must not occur between a command and subcommand. Nor may a category appear after a subcommand. `kubectl` is sometimes referenced as a case where category follows subcommand (`kubectl get pods`). However, this is a misunderstanding; `pods` is an argument.

#### Naming Categories and Subcommands

Names should be complete words. In some cases, it may be better to use a commonly accepted abbreviation (`id` instead of `identification`) if that abbreviation has widespread cultural currency. Words should be descriptive of the task or category they describe.

#### Shortened Categories and Subcommands

While some names are _descriptive_, they may also be cumbersome to type: `helm dependency update`. In such cases, the words may support _alternative shorthand_ notations: `helm dep up`. Shorthand notations _must_ be two characters or more, and _must_ be mnemonic to the longhand representation. `dependency`, for example, could be `dep` or `dp`, but can not be `req`. If two subcommands in the same category begin with the same sequence of letters, the shorthand for one ought not be the sequence of shared characters. For example, if `helm upgrade` and `helm update` are both commands, `up` ought not be the shorthand for either (though `upg` and `upd` would be okay).

### Options
As the name implies, there is no such thing as a _required option_. Any command that accepts options must be able to perform its central task without any options specified.

Option naming in the old UNIX tradition was done with a two-character sequence: a hyphen followed by a single ASCII letter or number: `-s`, `-9`. The GNU tools standardized on a _long option_ form where two hyphens preceded a sequence of letters and numbers: `--long`, `--dry-run`. The Plan9 system followed a convention similar to the GNU tools, but with only one hyphen: '-run', '-link'.

Helm allows all three conventions, but favors them in a particular order:

- All options must have a long form: `--dry-run`, `--quiet`.
- Options that are frequently used may have a short form: `-q`, `-v`, `-f`.
- The Plan9 format is automatically supported because it is part of the Go libraries for parsing.

Certain single-letter flags have a conventional meaning that Helm programs must adhere to:

- `-v` means _verbose_, and controls the amount of output sent to the terminal.
- `-q` means _quiet_, and minimizes the amount of output sent to the terminal.
- `-f` means _file_ and always takes a string argument that is a path to a file.
- `-h` means _help_ and displays help text.

Additionally, Helm reserves the long flags `--dry-run` and `--debug`:

- `--dry-run` means that an operation is performed only to the point where it does not have an impact on the system. A dry run install may stage an installation, but must not perform any installation of any components. A dry run delete may retrieve the record that would be deleted and verify that the record exists, but must not delete the record.
- `--debug` presents information that is useful to the user in learning about their own code. It _may_ be abbreviated with the `-v` flag since operationally they may be the same.

#### Compound Option

In rare cases, it may be necessary to support an option that takes "more options". In such cases, the following pattern is strongly suggested:

`--option suboption=value,suboption2=value`

This style is visually intuitive (to the extent possible), easily parsed, and can be combined easily with other options.

Suboptions should only be used when the contents contain a cohesive set of data whose possible values cannot be clearly captured as standard options. Example: `helm install --set key=value`.

#### Environment Variable Shadowing

Some options may be "shadowed" by environment variables. This may be done when an option is likely to be used repeatedly with the same value over a long period of time.

Environment variable shadowing order must be:

1. An explicit option is always given priority
2. If no explicit option is present, an environment variable value is checked
3. If the environment variable value is empty, the default value is used

In cases where a configuration file also shadows, it is to be consulted after environment variables, but before the default value.

### Arguments

Arguments represent pieces of data that are necessary for a program to run.

- `helm install stable/nginx`: COMMAND SUBCOMMAND ARGUMENT
- `helm upgrade sad-panda stable/nginx`: COMMAND SUBCOMMAND ARGUMENT ARGUMENT

Arguments are often expanded by the OS shell (e.g. `ls *.tgz`). This should be taken into consideration when designing a program.

Arguments are:

- Treated as variables: their content cannot be predicted.
- Variatic: Zero or more may be provided. The program must respond appropriately when the wrong number are provided.

In most case, Helm suggests that one of the following two patterns be used when designing argument handling:

1. A fixed number of arguments (typically between zero and three) are supported. Each argument may be of a different type (`helm upgrade my-release my-chart`)
2. A variadic number of arguments (one or more) are supported. Each argument must be of the same type. (`helm package chart1 chart2 chart3`)

#### Category or Argument?

When ought a noun be expressed as a category, and when ought it be expressed as an argument?

- Categories represent logical groupings of functionality
- Arguments represent objects that are to be acted upon by a command

One easy test for determining whether to use a category or an argument is to determine what the help text experience ought to be for a user:

- `helm repository --help` should list all of the commands that have to do with repositories

It would not make sense, for example, to have `helm nginx search`, as `nginx` is a thing being searched for, not a category under which we can group commands. As such `helm nginx --help` also feels incorrect.

## Runtime Activity (STDIN, STDOUT, STDERR)

There are well-established conventions for STDIN, STDOUT, and STDERR. Users supply input on STDIN, the program supplies information on STDOUT, and the program reports error descriptions on STDERR.

In addition to that, Helm makes the following recommendations.

**STDIN:** May be used for interactive prompts. However, interactive prompts ought to occur under only two conditions: (a) the operation is inherently destructive in a possibly unforeseen way OR (b) the user has supplied an option that indicates that they want to be prompted interactively.

As for accepting data on STDIN, it is preferred that most data be read through files (using options) and only accepts STDIN data when file-based processing is inadequate. This recommendation is primarily made for the sake of the user, for whom it is easier to document the file-based case than the STDIN-based case.

**STDOUT:** May be used for any user-facing or shell-facing output except for error messages. Warnings and debug messages _may_ be printed to STDOUT.

**STDERR:** Must only be used for error messages, warning messages, and verbose debugging information, as might be found in error logs.

### Preferred Formatting

**List-like** data should be presented as either a table or a single-column list. Tables are preferred.

```
$ helm search h
NAME                   	VERSION	DESCRIPTION
mumoshu/memcached      	0.1.0  	A simple Memcached cluster
kube-charts/drupal     	0.3.1  	One of the most versatile open source content m...
kube-charts/jenkins    	0.1.0  	A Jenkins Helm chart for Kubernetes.
kube-charts/mariadb    	0.4.0  	Chart for MariaDB
kube-charts/mysql      	0.1.0  	Chart for MySQL
kube-charts/redmine    	0.3.1  	A flexible project management web application.
kube-charts/wordpress  	0.3.0  	Web publishing platform for building blogs and ...
mumoshu/mysql          	0.2.0  	Chart running MySQL.
```

**Field data** where multiple fields are displayed, and each field has a key and some arbitrarily large data should be displayed as follows:

- If the data is only one line, the format should be `KEY: data`.
- If the data is multiple lines, it should be:

```
KEY:
Data
```

**Structured data** like JSON and YAML should be printed with correct indenting and line breaks so that they can be copied and pasted (or piped) into a processor.

**Log-like** data should be printed either one line per message, or one line aligned left, followed by multiple lines indented with a single tab. Error messages may follow this pattern.

If log-like data is intermixed with field or structured data, log messages should be prepended with `=> `

### Progress Meters

No progress metering should display unless the task will take, on average 3 seconds or more. (This is longer than the wait period in graphical UIs, which may be displayed at 1 second or more.)

For determinate tasks, a linear fill progress meter may be used. In all other cases, an indeterminate progress meter should be used.

Linear example:

```
[=================>      ]  75% complete
```

Indeterminate example:

```
[    <====>              ]  Downloading...
```

At no point should the user be left for more than 10 seconds with no change to the text on the screen. Users are quick to judge this as "failure", "hanging", or "getting stuck".

### Error and Warning Messages

An _Error_ is a negative event in the program that caused it to _cease processing_ without successful completion.

A _Warning_ is a negative event in the program that may have some side effects, but did not cause the program to cease.

Both error messages and warning messages should be printed to the screen.

#### Warning Messages

Warning messages should be preceded by the word `WARNING:` in all caps (optionally colored yellow). The message should contain at least one sentence, with the first character in lower case (unless it is a proper noun). Messages should be properly punctuated, though a trailing period may be omitted.

```
WARNING: download of file nginx.tgz failed. Continuing.
```

The message should provide the user with enough information that:

- they understand basically what went wrong
- the information can provide useful information when included in support requests

Warnings should not be vacuous. They must indicate what the problem is. `WARNING: something went wrong downloading` is not helpful to the user, or helpful for diagnosis.

#### Error Messages

Error messages occur only when the program terminates early. Errors begin with the word `Error:`. Note that this is not in all caps because it always appears at the end of the output. Error messages may be colored red.

As with warnings, error messages must contain at least one sentence. The first letter should not be capitalized unless it is a proper noun. When an error embeds a lower-level error message, the lower level error message should be offset with a comma. For long error messages, the lower level messages should be indented with a single tab.

```
Error: upload of `nginx.tgz` failed
```

With an embedded error:

```
Error: upload of `nginx.tgz` failed: file not found
```

With a long error:

```
Error: upload of `nginx.tgz` failed: 
	Dial failed for remote host 192.168.1.11: network not reachable
```

As with a warning, and error message must:

- convey an accurate message about what went wrong
- provide information specific enough that it is useful for debugging

#### Errors, Warnings, and Debugging

Errors and warnings must not be hidden behind debug flags (e.g. an error is only displayed if `--debug` is set).

`--debug` flags may be used to:

- determine when to suppress/show stack traces
- determine when to show additional state information
- determine when to show nested error messages that do not change the central meaning of the error message

It is considered poor form to have a default error experience that conveys different meaning than a debug-context error message.

BAD:

```
$ foo 
Error: failed to complete job
```

```
$ foo --debug
Error: Bootstrap sequence failed because network device was not ready
```

Debug messages may _augment_ the error, but not _supplant_ it.

GOOD:

```
$ foo
Error: failed to open `bar.txt`
```

```
$ foo --debug
Error: failed to open `bar.txt`: stat: `bar.txt` is write only
```

## The Output (Exit)

When a Helm program exits, it must return 0 on success. Errors must return between 1 and 125. The error codes 126 and 127 ought only be used to signal "command not found" and "command not executable", respectively.

## Help Text

The help text mantra for Helm is: *the help text is the primary documentation*. Reading the help text should be sufficient for understanding how to use the program.

Every command should be accompanied by help text. For the most part, the tools handle formatting. However the content of the help text is the domain of the developer.

The top-level help text for `helm` looks something like this:

```
The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

       	$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Common actions from this point include:

- helm search:    search for charts
- helm fetch:     download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment:
  $HELM_HOME      set an alternative location for Helm files. By default, these are stored in ~/.helm
  $HELM_HOST      set an alternative Tiller host. The format is host:port
  $KUBECONFIG     set an alternate Kubernetes configuration file (default "~/.kube/config")

Usage:
  helm [command]

Available Commands:
  create      create a new chart with the given name
  delete      given a release name, delete the release from Kubernetes
  ...         removed for conciseness
  verify      verify that a chart at the given path has been signed and is valid
  version     print the client/server version information

Flags:
      --debug         enable verbose output
      --home string   location of your Helm config. Overrides $HELM_HOME. (default "~/.helm")
      --host string   address of tiller. Overrides $HELM_HOST. (default "localhost:44134")

Use "helm [command] --help" for more information about a command.
```

### Long Descriptions

Each program, category, and command may have help text. And each help text begins with a _long description_.

This description must begin with a single sentence or phrase summarizing the feature. It should then be followed by one or more paragraphs describing how the tool is to be operated. This section may contain:

- Descriptions of what the program does or how it is to be invoked
- Descriptions of error or warning output
- Special instructions about options (such as side-effects of combining flags)
- Examples of usage
- Documentation of any environment variables that may have an impact on this tool

The long description may be divided into sections. A section should begin with a title where the fist letter of the title is capitalized, and the title ends with a colon (`:`). Above, the `Environment:` section is an example.

### Usage

Usage should always be of the form `command [flags] [category [subcommand]] [arguments]`.

### Short Descriptions

Subcommands, categories, and flags all have short descriptions. A short description should always begin with a lowercase letter, and should not end with a period. (Why this grammatical nonchalance? The precedent was set by many other tools, including `kubectl`.) It should explain in one to two sentences what feature is provided.

## Logging

During execution, programs may write log data. The destination of the log data is often determined by runtime configuration, and it may go to a log file, STDOUT, a network socket, or another destination.

When displayed to the user:

- Log lines must start with a date/time stamp
- Messages must include a severity marker. Minimally, severity markers are ERROR, WARNING, INFO, and DEBUG
- Messages should be phrases or sentences
- Internal markers like codes may be used in log messages
- Where possible, log messages should be kept to one line (approximately 90 characters)