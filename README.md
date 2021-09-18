# Devtools for developing themes

Here are the steps to use this tool:

1. Create a personal token with your Typlog accounts
2. Find out one of your Typlog site ID

For example, you have created an API token: `pt_axOz97`, and your site ID is `123`.
Head over to  your theme repo:

```
$ cd my-theme
$ ls
home.j2  item.j2  list.j2  style.css  theme.json
```

Then run this command in your theme folder:

```
$ SITE=123 TOKEN=pt_axOz97 serve-theme
```

Open your browser and visit `http://localhost:7000/`, it would render your `home.j2`.
