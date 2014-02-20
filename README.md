## satsuma

satsuma is the main component of [joinmytalk.com](https://joinmytalk.com/). 
Join my Talk! is a website to present everywhere you have an HTML5-capable web 
browser available. It doesn't require any installed software, Flash, or any browser 
plugins.

The audience can follow your talk, see exactly what you show on the screen, and read any 
additional notes that you provide with your slides. In the room you present, or remotely 
via the internet.

### Building satsuma

Make sure you have a working [Go](http://golang.org/) build environment.

Just use `go build` in the root directory and the `pdfd` subdirectory.

Also, run `bower update` in `htdocs/assets/js`.

### Running satsuma

In order to run satsuma, you require the following components:

* A MySQL database instance, bootstrapped with the SQL sources found the `sql` subdirectory.

* Redis

* LibreOffice and `unoconv` installed

* [NSQ](https://github.com/bitly/nsq) with nsqd and nsqlookupd running

* OAuth Client ID and Secret for Google+

* OAuth Client Key and Secret for Twitter

### License

For license information, please see the file `LICENSE.md`.


### Author

Andreas Krennmair <ak@synflood.at>
