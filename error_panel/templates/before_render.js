import("https://esm.sh/markdown-it@14").then(({ default: MarkdownIt }) => {
  window.markdownIt = new MarkdownIt();;
});
