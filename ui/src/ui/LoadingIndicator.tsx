function LoadingIndicator() {
  return (
    <div className="w-full text-center mx-auto flex items-center justify-center p-4">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-muted-foreground border-t-transparent"></div>
    </div>
  );
}

export default LoadingIndicator;
