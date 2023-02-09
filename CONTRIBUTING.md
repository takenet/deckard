# Contribute

You can help this project by reporting problems, suggestions, localizing it or contributing to the code.

## Report a problem or suggestion

Go to our [issue tracker](https://github.com/takenet/deckard/issues) and check if your problem/suggestion is already reported. If not, create a new issue with a descriptive title and detail your suggestion or steps to reproduce the problem.

## Contribute to the code

To contribute with Deckard you need to follow these steps:

* Fork this repo using the button at the top.
* Clone your forked repo locally.

``shell
git clone git@github.com:yourname/deckard.git``
```

* Always create a new issue when you plan to work on a bug or new feature and wait for other devs input before start coding.
* Once the new feature is approved or the problem confirmed, go to your local copy and create a new branch to work on it. Use a descriptive name for it, include the issue number for reference.

```shell
git checkout -b message-title-3
```

* Do your coding and push it to your fork. Include as few commits as possible (one should be enough) and a good description. Always include a reference to the issue you are working on.

We currently use [Conventionnal Commits](https://www.conventionalcommits.org/en/v1.0.0/) to write our commit messages and our Pull Request title. This is a good practice to keep our commit history clean and easy to read.

```
$ git add .
$ git commit -m "feat(3): Adds a new title field to the message"
$ git push origin message-title-3
```

* Do a new pull request from your "message-title-3" branch to deckard "main" branch. Use the same title as the commit message and include a description of the changes you made.

### Tests

We have unit tests for most of the code. To run them, use the following command:

```shell
make unit-test
```

Integration tests are also available. To run them, use the following command:

```shell
make integration-test
```

> Check on the [README](README.md) file for more information about how to run the integration tests.

To run all tests, use the following command:

```shell
make test
```

### How to keep your local branches updated

To keep your local main branch updated with upstream main, regularly do:

```
$ git fetch upstream
$ git checkout main
$ git pull --rebase upstream main
```

To update the branch you are coding in:

```
$ git checkout message-title-3
$ git rebase main
```

## Versioning

We use [Semantic Versioning](http://semver.org/) for versioning. To list all versions available, see the [tags on this repository](https://github.com/takenet/deckard/tags).

Our pre-release versions are named using the following pattern: `0.0.0-SNAPSHOT` and the `main` branch will always have a `SNAPSHOT` version. The `SNAPSHOT` stands for a code that was not released yet.

> Our tags are named using the following pattern: `v0.0.0`.

## Licesing

We currently use [MIT](LICENSE) license. Be careful to not include any code or dependency that is not compatible with this license.