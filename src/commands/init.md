# batsman init

`batsman init` intitializes a new sample site at the specified path. 
The path should not already exist, or should be empty if it already exists.

The format for the command is:

```
batsman init path/to/new/site
```

For example, running:

```
batsman init mysite
```
initializes a new site with example content in the `mysite` directory.


The source for the site lives in a directory named `src`. To generate the site,
run [`batsman build`](/commands/build). This will create a directory named `build` 
in the same directory as `src` that contains the generated static content.
