# batsman new

`batsman new` prints markdown front matter to stdout.
The format for the command is:

```
batsman [-title=<title>] [-draft] new
```

For example, 

```
batsman -title Foo new
```

prints to stdout:

```
+++
title = "Foo"
time  = "2016-08-28 23:44:02 -05:00"
+++
```

Typically you would redirect the output to a file:

```
batsman -title Foo new > src/blog/my-new-post.md
```
