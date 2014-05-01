# GoRazor

GoRazor is the Go port of the razor template engine originated from [asp.net world](http://weblogs.asp.net/scottgu/archive/2010/07/02/introducing-razor.aspx).

This port is essentially a re-port from razor's port in javascript: [vash](https://github.com/kirbysayshi/vash).

It modifies on vast's generation function to emit go code instead of javascript code.

In summay, GoRazor is:

* Consice syntax
* Able to mix go code in template
  * import is supported
  * Call arbitrary functions
* Take code generation approach, i.e. no reflection overhead
* Strong type template model
* Utils class

## Example

Here is a simple example of [gorazor templates](https://github.com/Wuvist/gorazor/tree/master/tpl) and the corresponding [generated codes](https://github.com/Wuvist/gorazor/tree/master/gen).

# Syntax

## Variable

* `@variable` to insert **string** variable into html template
  * variable should be wrapped by arbitary go functions
  * All variable inserted will be automatcially [esacped](http://golang.org/pkg/html/template/#HTMLEscapeString)

```html
<div>Hello @user.Name</div>
```

```html
<div>Hello @strings.ToUpper(req.CurrentUser.Name)</div>
```

## Flow Control

```php
@if .... {
	....
}

@if .... {
	....
} else {
	....
}

@for .... {
	
}

@{
	switch .... {
	case ....:
	      <p>...</p>
	case 2:
	      <p>...</p>
	default:
	      <p>...</p>
	}
}
```

Please use [example](https://github.com/Wuvist/gorazor/blob/master/tpl/main.gohtml) for reference.

## Code block

It's possbile to insert arbitary go code block in the template, like create new variable.

```html
@{
	username := u.Name
	if u.Email != "" {
		username += "(" + u.Email + ")"
	}
}
<div class="welcome">
<h4>Hello @username</h4>
</div>
```

It's recommendation to keep clean separation of code & view. Please consider move logic into your code before creating a code block in template.

## Decleration

The **first code block** in template is strictly for decleration:

* imports
* model type
* layout (Not supported yet, but soon)

like:

```
@{
	import  (
		"kp/models"
	)
	var totalMessage int
	var u *kpm.User
}
....
```

**first code block** must be at the begining of the template, i.e. before any html.

Any other codes inside the first code block will **be ignored**.

import must be wrapped in `()`, `import "packagename"` is not yet supported.

The variables declared in **first code block** will be the models of the template, they will become the parameters of generated function.

If your template doesn't need any model input, then just leave it blank.

## Helper / Include other template

As gorazor compiles tempaltes to go function, embedding another template is just calling the generated function, like any other go function.

However, if the template are designed to be embeded, it must be under `helper` namespace, i.e. put them in `helper` sub-folder. 

So, using a helper template is similar to:

```html

@if msg != "" {
	<div>@helper.ShowMsg(msg)</div>
}

```

GoRazor won't HTML escape the output of `helper.XXX`.

Please use [example](https://github.com/Wuvist/gorazor/blob/master/tpl/main.gohtml) for reference.

## Layout

TBA

# Rules

## Naming

* Template **folder name** will be used as **package name** in generated code
* Template file name must has the extension name `.gohtml`
* Template strip of `.gohtml` extension name will be used as the **function name** in generated code, with **fisrt letter Capitalized**.
  * So that the function will be accessible to other modules. (I hate GO about this.)
* Helper templates **must** has the package name **helper**

# FAQ

TBA

# Bugs

* `@switch .... {` is buggy, plese use `@{ switch ....`
* closure is code block is buggy

# Todo

* Refactor all the quick & dirty code
* Test suite
* Add tools, like monitor template changes and auto re-generate
* Performance benchmark
* Generate more function overloads, like accept additional buffer parameter for write
* Support direct usage of int/date variables in tempate?
  * i.e. use @user.Level directly, instead of @gorazor.Itoa(user.Level)