// Example post-navigation script
console.log('Post-navigation script executing...');
if (window.testData) {
    window.testData.loaded = Date.now();
    window.testData.duration = window.testData.loaded - window.testData.started;
    console.log('Page load duration:', window.testData.duration, 'ms');
} else {
    console.log('Warning: testData not found from pre-script');
}
console.log('Page title:', document.title);
console.log('Post-navigation script completed');