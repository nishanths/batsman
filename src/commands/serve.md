# batsman serve

`batsman serve` serves the `build`
directory on HTTP at the address specified by `-http`. The format of the command
is:

```
batsman [-http=<addr>] [-watch] serve
```

Both flags are optional. `-http` defaults to `localhost:8080` and `-watch` defaults to false.

If `-watch` is specified, then the `src` directory is watched recursively and the `build` directory
regenerated on changes to `src`.

Sidenote: The command regenerates the `build` directory once at startup, regardless of `-watch`.
