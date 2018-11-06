flaclink - a FLAC album linking service
=======================================

.. image:: https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg
   :target: https://opensource.org/licenses/MPL-2.0

flaclink is a local Go service to automatically link downloaded FLAC albums to a directory of your choosing. flaclink uses hardlinks, so it's compatibile with Plex and other media servers. The sample systemd service runs every 12-15 minutes, but you can configure it to run as often as you'd like.

flaclink uses bbolt_, an actively-maintained fork of the pure Go BoltDB_ embedded key/value store. bbolt is released under the `MIT License`_.

.. _bbolt: https://github.com/etc-io/bbolt
.. _BoltDB: https://github.com/boltdb/bolt
.. _MIT License: https://github.com/etcd-io/bbolt/blob/master/LICENSE

Installation
-------------
To install the executable, use:

.. code-block:: go

   go get -u https://github.com/kylegentle/flaclink

Next, to install flaclink as a service, create the ``flaclink.service`` and ``flaclink.timer`` unit files under ``/etc/systemd/system``. Sample units are provided in this repository for your reference; you'll need to modify them to fit your environment.

Then, run:

.. code-block:: bash

   # systemctl enable flaclink.timer
   # systemctl start flaclink.timer

And you're all set!


Command-Line Usage
-------------------
After installation, you can run the flaclink executable by itself to run the service a single time:

.. code-block:: bash

   flaclink <source_dir> <target_dir>

